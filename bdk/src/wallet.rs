use crate::types::*;
use anyhow::Context;
use bdk_electrum::{electrum_client, BdkElectrumClient};
use bdk_wallet::bitcoin::secp256k1::Secp256k1;
use bdk_wallet::{
    bitcoin::{bip32::DerivationPath, Network as BdkNetwork},
    chain::ChainPosition,
    keys::{any_network, bip39::Mnemonic, DerivableKey, DescriptorSecretKey, ExtendedKey},
    rusqlite::Connection,
    template::DescriptorTemplateOut,
    KeychainKind, PersistedWallet, SignOptions, Wallet as BdkWallet,
};
use miniscript::{
    bitcoin::{
        bip32::{ChildNumber, Xpriv, Xpub},
        consensus::encode::{self, deserialize_hex},
        Address, Amount, FeeRate, Transaction, Txid,
    },
    descriptor::{DerivPaths, DescriptorMultiXKey, DescriptorXKey, Wildcard, Wpkh},
    DescriptorPublicKey, ForEachKey,
};
use std::{
    str::FromStr,
    sync::{Arc, Mutex, MutexGuard},
};

fn mnemonic_to_xprv(mnemonic: String, network: BdkNetwork) -> Result<Xpriv, anyhow::Error> {
    let mnemonic = Mnemonic::from_str(&mnemonic)?;
    let xkey: ExtendedKey = mnemonic.into_extended_key()?;
    let xprv = xkey
        .into_xprv(network)
        .ok_or(anyhow::anyhow!("no private key"))?;
    Ok(xprv)
}

fn derive_xpriv(
    public_key: DescriptorPublicKey,
    xprv: Xpriv,
) -> Result<DescriptorSecretKey, anyhow::Error> {
    let secp = Secp256k1::new();
    match public_key {
        DescriptorPublicKey::XPub(x) => match x.clone().origin.clone() {
            Some((fingerprint, origin_path)) => {
                if fingerprint == xprv.fingerprint(&secp) {
                    let xpriv = xprv.derive_priv(&secp, &origin_path)?;
                    Ok(DescriptorSecretKey::XPrv(DescriptorXKey::<Xpriv> {
                        origin: x.origin.clone(),
                        xkey: xpriv,
                        derivation_path: x.derivation_path,
                        wildcard: x.wildcard,
                    }))
                } else {
                    Err(anyhow::anyhow!("fingerprint does not match"))
                }
            }
            None => Err(anyhow::anyhow!("no origin")),
        },
        _ => Err(anyhow::anyhow!("unsupported key")),
    }
}

#[uniffi::export]
fn derive_default_xpub(network: Network, mnemonic: String) -> Result<String, Error> {
    let secp = Secp256k1::new();
    let derivation_path = DerivationPath::from_str("m/84h/0h/0h").context("derivation path")?;

    let xprv = mnemonic_to_xprv(mnemonic, network.into())?;
    let derived = xprv
        .derive_priv(&secp, &derivation_path)
        .context("derive priv")?;

    let multi = DescriptorMultiXKey {
        xkey: Xpub::from_priv(&secp, &derived),
        origin: Some((xprv.fingerprint(&secp), derivation_path.clone())),
        derivation_paths: DerivPaths::new(vec![
            DerivationPath::from_iter(vec![ChildNumber::Normal { index: 0 }]),
            DerivationPath::from_iter(vec![ChildNumber::Normal { index: 1 }]),
        ])
        .context("derivation paths")?,
        wildcard: Wildcard::Unhardened,
    };

    let descriptor_pubkey = DescriptorPublicKey::MultiXPub(multi);
    Ok(Wpkh::new(descriptor_pubkey).context("wpkh")?.to_string())
}

#[derive(uniffi::Record)]
pub struct WalletCredentials {
    pub mnemonic: Option<String>,
    pub core_descriptor: String,
}

impl WalletCredentials {
    pub fn get_wallet_descriptor(
        &self,
        chain: KeychainKind,
        network: BdkNetwork,
    ) -> anyhow::Result<Option<DescriptorTemplateOut>> {
        let secp = Secp256k1::new();

        let (public, mut keymap) = miniscript::descriptor::Descriptor::parse_descriptor(
            &secp,
            self.core_descriptor.as_str(),
        )?;

        let descriptors = public.clone().into_single_descriptors()?;
        if descriptors.len() > 2 {
            return Err(anyhow::anyhow!("multiple descriptors"));
        }

        descriptors
            .get(chain as usize)
            .map(|descriptor| {
                if keymap.is_empty() {
                    if let Some(mnemonic) = &self.mnemonic {
                        let xprv = mnemonic_to_xprv(mnemonic.clone(), network)?;

                        let mut public_keys = vec![];
                        descriptor.for_each_key(|d| {
                            public_keys.push(d.clone());
                            true
                        });

                        for public_key in public_keys {
                            keymap.insert(public_key.clone(), derive_xpriv(public_key, xprv)?);
                        }
                    };
                }
                Ok((descriptor.clone(), keymap, any_network()))
            })
            .transpose()
    }
}

#[derive(uniffi::Object)]
pub struct ChainClient {
    pub(crate) electrum: BdkElectrumClient<electrum_client::Client>,
}

#[uniffi::export]
impl ChainClient {
    #[uniffi::constructor]
    pub fn new(electrum_url: String) -> Result<Self, Error> {
        let client = electrum_client::Client::new(electrum_url.as_str())
            .context("create electrum client")?;
        Ok(Self {
            electrum: BdkElectrumClient::new(client),
        })
    }
}

fn parse_fee_rate(sat_per_vbyte: f64) -> FeeRate {
    FeeRate::from_sat_per_kwu((sat_per_vbyte * 1000.0 / 4.0) as u64)
}

#[derive(uniffi::Object)]
pub struct Wallet {
    inner: Mutex<PersistedWallet<Connection>>,
    db: Mutex<Connection>,
    network: BdkNetwork,
}

#[uniffi::export]
impl Wallet {
    #[uniffi::constructor]
    pub fn new(
        credentials: WalletCredentials,
        db_path: String,
        network: Network,
    ) -> Result<Self, Error> {
        let network: BdkNetwork = network.into();
        let external = credentials
            .get_wallet_descriptor(KeychainKind::External, network)?
            .ok_or(Error::Generic(
                "external descriptor is required".to_string(),
            ))?;
        let internal = credentials.get_wallet_descriptor(KeychainKind::Internal, network)?;
        let mut db = Connection::open(db_path.as_str()).context("open db")?;
        let wallet_opt = BdkWallet::load()
            .descriptor(KeychainKind::External, Some(external.clone()))
            .descriptor(KeychainKind::Internal, internal.clone())
            .extract_keys()
            .check_network(network)
            .load_wallet(&mut db)
            .context("load wallet")?;
        let wallet = match wallet_opt {
            Some(wallet) => wallet,
            None => {
                let params = match internal {
                    Some(internal) => BdkWallet::create(external, internal),
                    None => BdkWallet::create_single(external),
                };
                params
                    .network(network)
                    .create_wallet(&mut db)
                    .context("create wallet")?
            }
        };

        Ok(Self {
            inner: Mutex::new(wallet),
            db: Mutex::new(db),
            network,
        })
    }

    pub fn full_scan(&self, chain_client: Arc<ChainClient>) -> Result<(), Error> {
        let mut wallet = self.get_wallet()?;
        let request = wallet.start_full_scan();
        let update = chain_client
            .electrum
            .full_scan(request, 20, 50, false)
            .context("full scan")?;
        wallet.apply_update(update).context("apply update")?;
        self.persist(&mut wallet)?;
        Ok(())
    }

    pub fn sync(&self, chain_client: Arc<ChainClient>) -> Result<(), Error> {
        let mut wallet = self.get_wallet()?;
        let request = wallet.start_sync_with_revealed_spks();
        let update = chain_client
            .electrum
            .sync(request, 50, false)
            .context("sync")?;
        wallet.apply_update(update).context("apply update")?;
        self.persist(&mut wallet)?;
        Ok(())
    }

    pub fn apply_transaction(&self, tx_hex: String) -> Result<(), Error> {
        let mut wallet = self.get_wallet()?;
        let tx: Transaction = deserialize_hex(tx_hex.as_str()).context("deserialize tx")?;
        wallet.apply_unconfirmed_txs(vec![(Arc::new(tx), 0)]);
        self.persist(&mut wallet)?;
        Ok(())
    }

    pub fn bump_transaction_fee(&self, tx_id: String, sat_per_vbyte: f64) -> Result<String, Error> {
        let tx_id = Txid::from_str(&tx_id).context("parse tx id")?;
        let fee_rate = parse_fee_rate(sat_per_vbyte);

        let mut wallet = self.get_wallet()?;
        let mut builder = wallet.build_fee_bump(tx_id).context("build fee bump")?;
        builder.fee_rate(fee_rate);

        let mut psbt = builder.finish().context("finish")?;
        wallet
            .sign(&mut psbt, SignOptions::default())
            .context("sign tx")?;

        let tx = psbt.extract_tx().context("extract tx")?;
        Ok(encode::serialize_hex(&tx))
    }

    pub fn new_address(&self) -> Result<String, Error> {
        let mut wallet = self.get_wallet()?;
        let result = wallet.reveal_next_address(KeychainKind::External);
        self.persist(&mut wallet)?;
        Ok(result.address.to_string())
    }

    pub fn balance(&self) -> Result<Balance, Error> {
        let wallet = self.get_wallet()?;
        let balance = wallet.balance();
        Ok(Balance {
            confirmed: balance.confirmed.to_sat(),
            unconfirmed: balance.trusted_pending.to_sat() + balance.untrusted_pending.to_sat(),
        })
    }

    pub fn send_to_address(
        &self,
        address: String,
        amount: u64,
        sat_per_vbyte: f64,
        send_all: bool,
    ) -> Result<WalletSendResult, Error> {
        let fee_rate = parse_fee_rate(sat_per_vbyte);
        let address = Address::from_str(&address)
            .context("parse address")?
            .require_network(self.network)
            .context("require network")?
            .script_pubkey();

        let mut wallet = self.get_wallet()?;
        let mut builder = wallet.build_tx();
        builder.fee_rate(fee_rate);
        if send_all {
            builder.drain_wallet().drain_to(address.clone());
        } else {
            builder.add_recipient(address.clone(), Amount::from_sat(amount));
        }

        let mut psbt = builder.finish().context("finish tx")?;
        wallet
            .sign(&mut psbt, SignOptions::default())
            .context("sign tx")?;

        let fee = psbt.fee().context("get fee")?.to_sat();
        let tx = psbt.extract_tx().context("extract tx")?;

        let send_amount = tx
            .output
            .iter()
            .find(|o| o.script_pubkey == address)
            .map(|o| o.value.to_sat())
            .unwrap_or_default();

        Ok(WalletSendResult {
            tx_hex: encode::serialize_hex(&tx),
            send_amount,
            fee,
        })
    }

    pub fn get_transactions(
        &self,
        limit: u64,
        offset: u64,
    ) -> Result<Vec<WalletTransaction>, Error> {
        let wallet = self.get_wallet()?;

        let transactions = wallet
            .transactions()
            .skip(offset as usize)
            .take(if limit > 0 {
                limit as usize
            } else {
                usize::MAX
            })
            .filter_map(|tx| wallet.tx_details(tx.tx_node.txid))
            .map(|details| WalletTransaction {
                id: details.txid.to_string(),
                timestamp: match details.chain_position {
                    ChainPosition::Confirmed { anchor, .. } => anchor.confirmation_time,
                    ChainPosition::Unconfirmed { last_seen, .. } => last_seen.unwrap_or_default(),
                },
                outputs: details
                    .tx
                    .output
                    .iter()
                    .map(|out| WalletTransactionOutput {
                        address: Address::from_script(&out.script_pubkey, &self.network)
                            .map(|a| a.to_string())
                            .unwrap_or_default(),
                        amount: out.value.to_sat(),
                        is_our_address: wallet
                            .derivation_of_spk(out.script_pubkey.clone())
                            .is_some(),
                    })
                    .collect(),
                block_height: match details.chain_position {
                    ChainPosition::Confirmed { anchor, .. } => anchor.block_id.height,
                    ChainPosition::Unconfirmed { .. } => 0,
                },
                balance_change: details.balance_delta.to_sat(),
                is_consolidation: false,
            })
            .collect();
        Ok(transactions)
    }
}

impl Wallet {
    fn get_wallet(&self) -> Result<MutexGuard<'_, PersistedWallet<Connection>>, Error> {
        self.inner.lock().map_err(|e| Error::Generic(e.to_string()))
    }

    fn persist(
        &self,
        guard: &mut MutexGuard<'_, PersistedWallet<Connection>>,
    ) -> Result<(), Error> {
        let mut db = self.db.lock().map_err(|e| Error::Generic(e.to_string()))?;
        guard.persist(&mut *db).context("persist")?;
        Ok(())
    }
}
