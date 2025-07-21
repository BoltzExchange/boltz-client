use std::str::FromStr;

use bdk_wallet::{
    bitcoin::{bip32::DerivationPath, Network},
    descriptor,
    keys::{bip39::Mnemonic, KeyMap},
    rusqlite::Connection,
    KeychainKind, LoadParams, PersistedWallet, Wallet as BdkWallet,
};

use bdk_wallet::bitcoin::secp256k1::Secp256k1;
use bdk_wallet::descriptor::IntoWalletDescriptor;
use miniscript::{
    bitcoin::{bip32::ChildNumber, consensus::encode, Address, Amount, FeeRate, PubkeyHash}, serde::Serialize, Descriptor, DescriptorPublicKey, ForEachKey
};

pub struct WalletCredentials {
    pub mnemonic: Option<String>,
    pub core_descriptor: Option<String>,
}

impl WalletCredentials {
    pub fn get_wallet_descriptor(
        &self,
        chain: KeychainKind,
    ) -> anyhow::Result<Option<(Descriptor<DescriptorPublicKey>, KeyMap)>> {
        if self.mnemonic.is_none() && self.core_descriptor.is_none() {
            return Err(anyhow::anyhow!("mnemonic or core descriptor is required"));
        }

        if self.mnemonic.is_some() && self.core_descriptor.is_some() {
            return Err(anyhow::anyhow!(
                "mnemonic and core descriptor cannot be used together"
            ));
        }

        if self.mnemonic.is_some() {
            let mnemonic = Mnemonic::from_str(&self.mnemonic.as_ref().unwrap()).unwrap();
            let mnemonic_with_passphrase = (mnemonic, None);
            let path = DerivationPath::from_str("m/84h/0h/0h")
                .unwrap()
                .child(ChildNumber::from_normal_idx(chain as u32)?);
            let result = descriptor!(wpkh((mnemonic_with_passphrase, path)))?;
            Ok(Some((result.0, result.1)))
        } else {
            let secp = Secp256k1::new();
            let (public, keymap) = miniscript::descriptor::Descriptor::parse_descriptor(
                &secp,
                self.core_descriptor.as_ref().unwrap(),
            )?;
            let mut descriptors = public.into_single_descriptors()?;
            if descriptors.len() > 2 {
                return Err(anyhow::anyhow!("unsupported descriptor"));
            }
            Ok(Some((descriptors.remove(chain as usize), keymap)))
        }
    }
}

pub struct Wallet {
    inner: PersistedWallet<Connection>,
    db: Connection,
    pub mnemonic: String,
}

impl Wallet {
    pub fn new(credentials: WalletCredentials) -> anyhow::Result<Self> {
        let NETWORK = Network::Bitcoin;
        let DB_PATH = "wallet.db";

        let external = credentials.get_wallet_descriptor(KeychainKind::External)?;
        let internal = credentials.get_wallet_descriptor(KeychainKind::Internal)?;

        let mut db = Connection::open(DB_PATH)?;
        let wallet_opt = BdkWallet::load()
            .descriptor(KeychainKind::External, external.clone())
            .descriptor(KeychainKind::Internal, internal.clone())
            .extract_keys()
            .check_network(NETWORK)
            .load_wallet(&mut db)?;
        let wallet = match wallet_opt {
            Some(wallet) => wallet,
            None => BdkWallet::create(external.unwrap(), internal.unwrap())
                .network(NETWORK)
                .create_wallet(&mut db)?,
        };

        //let electrum = BdkEl::new(electrum_url)?;
        Ok(Self {
            db,
            mnemonic: credentials.mnemonic.unwrap_or_default(),
            inner: wallet,
        })
    }

    pub fn sync(&mut self) -> anyhow::Result<()> {
        let request = self.inner.start_full_scan();
        self.inner.sync()?;
        Ok(())
    }

    pub fn address(&mut self) -> anyhow::Result<String> {
        let result = self.inner.reveal_next_address(KeychainKind::External);
        self.persist()?;
        Ok(result.address.to_string())
    }

    pub fn send_to_address(&mut self, address: String, amount: u64) -> anyhow::Result<()> {
        let address = Address::from_str(&address)?.assume_checked();
        let mut builder = self.inner.build_tx();
        builder
            .add_recipient(address, Amount::from_sat(amount))
            .fee_rate(FeeRate::from_sat_per_vb(10).ok_or(anyhow::anyhow!("invalid fee rate"))?);
        let result = builder.finish()?;

        let tx = result.extract_tx()?;


        let t = encode::serialize_hex(&tx);
        println!("tx: {:?}", t);

        self.persist()?;

        Ok(())
    }

    pub fn persist(&mut self) -> anyhow::Result<()> {
        self.inner.persist(&mut self.db)?;
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use miniscript::Descriptor;

    use super::*;

    #[test]
    fn it_works() {
        let credentials = WalletCredentials {
            mnemonic: Some(
                "test test test test test test test test test test test junk".to_string(),
            ),
            core_descriptor: None,
        };

        let mut w = Wallet::new(credentials).unwrap();

        let address = w.address().unwrap();
        println!("address: {:?}", address);

        /*
        let descriptor_credentials = WalletCredentials {
            mnemonic: None,
            core_descriptor: Some("test".to_string()),
        };
        let params = LoadParams::default();
        let _ = descriptor_credentials.load(params).unwrap();
        */

        let mnemonic =
            Mnemonic::from_str("test test test test test test test test test test test junk")
                .unwrap();
        let mnemonic_with_passphrase = (mnemonic, None);

        let external_path = DerivationPath::from_str("m/84h/0h/0h/0").unwrap();
        let internal_path = DerivationPath::from_str("m/84h/0h/0h/1").unwrap();

        let external =
            descriptor!(wpkh((mnemonic_with_passphrase.clone(), external_path))).unwrap();
        //let internal = descriptor!(wpkh((mnemonic_with_passphrase, internal_path))).unwrap();

        //println!("external: {:?}", external.0.clone().to_string());

        println!("external: {:?}", external.0.clone().to_string());

        let test = "wpkh([16a93ed0/84'/0'/0']xpub6C4SXHaqSMPK8MGL7bWjvS8WQnhoadsaXv2mnBcjAv142P2TivoC7s7ukNRYxETJZGc3JNzDKHQAGH5N3oQhQgkpwk6WrMtJB4eG2fpRAsV/<0;1>/*)#ad7pp59k";
        let secp = Secp256k1::new();
        let test = miniscript::descriptor::Descriptor::parse_descriptor(&secp, test).unwrap();

        let single = test.0.into_single_descriptors().unwrap();
        println!("single: {:?}", single);

        let t = external.0.clone();

        external.0.for_each_key(|k| {
            match k {
                DescriptorPublicKey::XPub(x) => {
                    println!("x: {:?}", x.derivation_path);
                }
                _ => {
                    println!("k: {:?}", k);
                }
            };
            true
        });
    }
}
