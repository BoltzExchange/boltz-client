extern crate lightning;
extern crate bech32;

use std::convert::TryFrom;
use std::ffi::{c_char, CStr};
use lightning::offers::offer::Amount;
use bech32::FromBase32;
use lightning::bitcoin::secp256k1::PublicKey;
use lightning::offers::invoice::Bolt12Invoice;

const BECH32_BOLT12_INVOICE_HRP: &str = "lni";

fn parse_offer(offer: *const c_char) -> Result<lightning::offers::offer::Offer, String> {
    let raw = unsafe { CStr::from_ptr(offer) };
    match raw.to_str() {
        Ok(res) => res.parse::<lightning::offers::offer::Offer>().map_err(|_| "Failed to parse offer".to_string()),
        Err(err) => Err(err.to_string()),
    }
}

#[no_mangle]
pub unsafe extern "C" fn decode_offer(offer: *const c_char) -> *mut Offer {
    match parse_offer(offer) {
        Ok(offer) =>
            Box::into_raw(Box::new(Offer {
                min_amount:
                match offer.amount() {
                    Some(amount) => match amount {
                        Amount::Bitcoin { amount_msats } => amount_msats / 1000,
                        Amount::Currency { .. } => 0,
                    },
                    None => 0,
                },
            }))
        ,
        Err(_) => std::ptr::null_mut(),
    }
}

fn parse_invoice(invoice: *const c_char) -> Result<Bolt12Invoice, String> {
    let raw = unsafe { CStr::from_ptr(invoice) };
    let invoice = match raw.to_str() {
        Ok(res) => res,
        Err(err) => return Err(err.to_string()),
    };
    let (hrp, data) = match bech32::decode_without_checksum(invoice) {
        Ok(res) => res,
        Err(err) => return Err(err.to_string()),
    };
    if hrp.as_str() != BECH32_BOLT12_INVOICE_HRP {
        return Err("Invalid HRP".to_string());
    }
    let data = match Vec::<u8>::from_base32(&data) {
        Ok(res) => res,
        Err(err) => return Err(err.to_string()),
    };
    match lightning::offers::invoice::Bolt12Invoice::try_from(data) {
        Ok(res) => Ok(res),
        Err(_) => Err("Failed to parse invoice".to_string()),
    }
}

#[no_mangle]
pub unsafe extern "C" fn decode_invoice(invoice: *const c_char) -> *mut Invoice {
    match parse_invoice(invoice) {
        Ok(invoice) => Box::into_raw(Box::new(Invoice {
            amount: invoice.amount_msats() / 1000,
            payment_hash: invoice.payment_hash().0,
            expiry_date: invoice.absolute_expiry().and_then(|d| Some(d.as_secs())).unwrap_or(0),
        })),
        Err(err) => std::ptr::null_mut(),
    }
}

#[no_mangle]
pub unsafe extern "C" fn check_invoice_offer(invoice: *const c_char, offer: *const c_char) -> bool {
    let invoice = match parse_invoice(invoice) {
        Ok(res) => res,
        Err(_) => return false,
    };
    let offer = match parse_offer(offer) {
        Ok(res) => res,
        Err(_) => return false,
    };
    let mut possible_signers: Vec<PublicKey> = Vec::new();
    offer.paths().iter().for_each(|path| {
        if let Some(last_signer) = path.blinded_hops().last() {
            possible_signers.push(last_signer.blinded_node_id);
        }
    });
    if let Some(signer) = offer.signing_pubkey() {
        possible_signers.push(signer);
    }
    possible_signers.contains(&invoice.signing_pubkey())
}

#[repr(C)]
pub struct Offer {
    pub min_amount: u64,
}

#[repr(C)]
pub struct Invoice {
    pub amount: u64,
    pub payment_hash: [u8; 32],
    pub expiry_date: u64,
