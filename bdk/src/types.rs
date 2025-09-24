use bdk_wallet::bitcoin::Network as BdkNetwork;

#[derive(uniffi::Error, thiserror::Error, Debug)]
pub enum Error {
    #[error("{0}")]
    Generic(String),
}

impl From<anyhow::Error> for Error {
    fn from(error: anyhow::Error) -> Self {
        // Use chain() to get the full error chain instead of just the top-level message
        let full_error = error
            .chain()
            .map(|e| e.to_string())
            .collect::<Vec<_>>()
            .join(": ");
        Error::Generic(full_error)
    }
}

#[derive(uniffi::Enum)]
pub enum Network {
    Bitcoin,
    Testnet,
    Regtest,
    Signet,
}

impl From<Network> for BdkNetwork {
    fn from(network: Network) -> Self {
        match network {
            Network::Bitcoin => BdkNetwork::Bitcoin,
            Network::Testnet => BdkNetwork::Testnet,
            Network::Regtest => BdkNetwork::Regtest,
            Network::Signet => BdkNetwork::Signet,
        }
    }
}

#[derive(uniffi::Record)]
pub struct Balance {
    pub confirmed: u64,
    pub unconfirmed: u64,
}

#[derive(uniffi::Record)]
pub struct WalletTransaction {
    pub id: String,
    pub timestamp: u64,
    pub outputs: Vec<WalletTransactionOutput>,
    pub block_height: u32,
    pub balance_change: i64,
    pub is_consolidation: bool,
}

#[derive(uniffi::Record)]
pub struct WalletTransactionOutput {
    pub address: String,
    pub amount: u64,
    pub is_our_address: bool,
}

#[derive(uniffi::Record)]
pub struct WalletSendResult {
    pub tx_hex: String,
    pub fee: u64,
    // the amount of sats that will be sent to the address
    pub send_amount: u64,
}
