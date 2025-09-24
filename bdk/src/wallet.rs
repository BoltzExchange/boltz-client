use crate::types::*;
use std::{
    str::FromStr,
    sync::{Arc, Mutex, MutexGuard},
};

use anyhow::Context;
use bdk_electrum::{electrum_client, BdkElectrumClient};
use bdk_wallet::{
    bitcoin::{bip32::DerivationPath, Network as BdkNetwork},
    chain::ChainPosition,
    descriptor,
    keys::{any_network, bip39::Mnemonic},
    rusqlite::Connection,
    template::DescriptorTemplateOut,
    KeychainKind, PersistedWallet, SignOptions, Wallet as BdkWallet,
};

use bdk_wallet::bitcoin::secp256k1::Secp256k1;
use miniscript::bitcoin::{
    bip32::ChildNumber,
    consensus::encode::{self, deserialize_hex},
    Address, Amount, FeeRate, Transaction, Txid,
};

#[derive(uniffi::Enum)]
pub enum WalletCredentials {
    Mnemonic(String),
    CoreDescriptor(String),
}

impl WalletCredentials {
    pub fn get_wallet_descriptor(
        &self,
        chain: KeychainKind,
    ) -> anyhow::Result<Option<DescriptorTemplateOut>> {
        match self {
            WalletCredentials::Mnemonic(mnemonic) => {
                let mnemonic = Mnemonic::from_str(mnemonic)?;
                let mnemonic_with_passphrase = (mnemonic, None);
                let path = DerivationPath::from_str("m/84h/0h/0h")
                    .unwrap()
                    .child(ChildNumber::from_normal_idx(chain as u32)?);
                Ok(Some(descriptor!(wpkh((mnemonic_with_passphrase, path)))?))
            }
            WalletCredentials::CoreDescriptor(core_descriptor) => {
                let secp = Secp256k1::new();
                let (public, keymap) =
                    miniscript::descriptor::Descriptor::parse_descriptor(&secp, core_descriptor)?;
                let descriptors = public.into_single_descriptors()?;
                if descriptors.len() > 2 {
                    return Err(anyhow::anyhow!("unsupported descriptor"));
                }
                Ok(descriptors
                    .get(chain as usize)
                    .map(|d| (d.clone(), keymap, any_network())))
            }
        }
    }
}

#[derive(uniffi::Object)]
pub struct Backend {
    pub(crate) electrum: BdkElectrumClient<electrum_client::Client>,
    pub(crate) network: BdkNetwork,
    pub(crate) db: Mutex<Connection>,
}

#[uniffi::export]
impl Backend {
    #[uniffi::constructor]
    pub fn new(network: Network, electrum_url: String, db_path: String) -> Result<Self, Error> {
        let client = electrum_client::Client::new(electrum_url.as_str())
            .context("create electrum client")?;
        let db = Connection::open(db_path.as_str()).context("open db")?;
        Ok(Self {
            electrum: BdkElectrumClient::new(client),
            network: network.into(),
            db: Mutex::new(db),
        })
    }
}

impl Backend {
    pub fn get_db(&self) -> Result<MutexGuard<'_, Connection>, Error> {
        self.db.lock().map_err(|e| Error::Generic(e.to_string()))
    }
}

#[derive(uniffi::Object)]
pub struct Wallet {
    inner: Mutex<PersistedWallet<Connection>>,
    backend: Arc<Backend>,
}

#[uniffi::export]
impl Wallet {
    #[uniffi::constructor]
    pub fn new(backend: Arc<Backend>, credentials: WalletCredentials) -> Result<Self, Error> {
        let external = credentials
            .get_wallet_descriptor(KeychainKind::External)?
            .ok_or(Error::Generic(
                "external descriptor is required".to_string(),
            ))?;
        let internal = credentials.get_wallet_descriptor(KeychainKind::Internal)?;

        let mut db = backend.get_db()?;
        let wallet_opt = BdkWallet::load()
            .descriptor(KeychainKind::External, Some(external.clone()))
            .descriptor(KeychainKind::Internal, internal.clone())
            .extract_keys()
            .check_network(backend.network)
            .load_wallet(&mut *db)
            .context("load wallet")?;
        let wallet = match wallet_opt {
            Some(wallet) => wallet,
            None => {
                let params = match internal {
                    Some(internal) => BdkWallet::create(external, internal),
                    None => BdkWallet::create_single(external),
                };
                params
                    .network(backend.network)
                    .create_wallet(&mut *db)
                    .context("create wallet")?
            }
        };

        Ok(Self {
            inner: Mutex::new(wallet),
            backend: backend.clone(),
        })
    }

    pub fn sync(&self) -> Result<(), Error> {
        let mut wallet = self.get_wallet()?;
        let request = wallet.start_full_scan();
        let update = self
            .backend
            .electrum
            .full_scan(request, 20, 50, false)
            .context("full scan")?;
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
        let fee_rate = FeeRate::from_sat_per_kwu((sat_per_vbyte * 1000.0) as u64);

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

    pub fn address(&self) -> Result<String, Error> {
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
        let fee_rate = FeeRate::from_sat_per_kwu((sat_per_vbyte * 1000.0) as u64);
        let address = Address::from_str(&address)
            .context("parse address")?
            .require_network(self.backend.network)
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
            .take(limit as usize)
            .skip(offset as usize)
            .map(|tx| {
                let tx_node = tx.tx_node;
                let details = wallet.tx_details(tx_node.txid).unwrap();

                WalletTransaction {
                    id: tx_node.txid.to_string(),
                    timestamp: match tx.chain_position {
                        ChainPosition::Confirmed { anchor, .. } => anchor.confirmation_time,
                        ChainPosition::Unconfirmed { last_seen, .. } => {
                            last_seen.unwrap_or_default()
                        }
                    },
                    outputs: tx_node
                        .tx
                        .output
                        .iter()
                        .map(|out| WalletTransactionOutput {
                            address: Address::from_script(
                                &out.script_pubkey,
                                &self.backend.network,
                            )
                            .map(|a| a.to_string())
                            .unwrap_or_default(),
                            amount: out.value.to_sat(),
                            is_our_address: wallet
                                .derivation_of_spk(out.script_pubkey.clone())
                                .is_some(),
                        })
                        .collect(),
                    block_height: match tx.chain_position {
                        ChainPosition::Confirmed { anchor, .. } => anchor.block_id.height,
                        ChainPosition::Unconfirmed { .. } => 0,
                    },
                    balance_change: details.balance_delta.to_sat(),
                    is_consolidation: false,
                }
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
        let mut db = self.backend.get_db()?;
        guard.persist(&mut *db).context("persist")?;
        Ok(())
    }
}
