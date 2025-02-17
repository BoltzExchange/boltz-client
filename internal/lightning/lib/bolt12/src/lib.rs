extern crate bech32;
extern crate lightning;

use bech32::FromBase32;
use lightning::bitcoin::secp256k1::PublicKey;
use lightning::offers::invoice::Bolt12Invoice;
use lightning::offers::offer::Amount;
use lightning::offers::offer::Offer as LnOffer;
use std::convert::TryFrom;
use std::ffi::{c_char, CStr, CString};
use std::ptr::null;

const BECH32_BOLT12_INVOICE_HRP: &str = "lni";

#[repr(C)]
pub struct Offer {
    pub min_amount_sat: u64,
}

#[repr(C)]
pub struct Invoice {
    pub amount_sat: u64,
    pub payment_hash: [u8; 32],
    pub expiry_date: u64,
}

#[repr(C)]
pub struct CResult<T> {
    pub result: *const T,
    pub error: *const c_char,
}

impl<T> CResult<T> {
    pub fn from_result(res: Result<T, String>) -> Self {
        match res {
            Ok(value) => CResult {
                result: Box::into_raw(Box::new(value)),
                error: null(),
            },
            Err(err) => CResult {
                result: null(),
                error: CString::new(err).unwrap_or_default().into_raw(),
            },
        }
    }
}

fn convert_msats_to_sats(msats: u64) -> u64 {
    ((msats as f64) / 1_000.0).ceil() as u64
}

fn parse_offer(offer: *const c_char) -> Result<LnOffer, String> {
    let raw = unsafe { CStr::from_ptr(offer) };
    match raw.to_str() {
        Ok(res) => res
            .parse::<LnOffer>()
            .map_err(|err| format!("{:?}", err).to_string()),
        Err(err) => Err(err.to_string()),
    }
}

#[no_mangle]
pub unsafe extern "C" fn decode_offer(offer: *const c_char) -> CResult<Offer> {
    CResult::from_result(parse_offer(offer).map(|offer| Offer {
        min_amount_sat: match offer.amount() {
            Some(amount) => match amount {
                Amount::Bitcoin { amount_msats } => convert_msats_to_sats(amount_msats),
                Amount::Currency { .. } => 0,
            },
            None => 0,
        },
    }))
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
        return Err("invalid HRP".to_string());
    }
    let data = match Vec::<u8>::from_base32(&data) {
        Ok(res) => res,
        Err(err) => return Err(err.to_string()),
    };
    match lightning::offers::invoice::Bolt12Invoice::try_from(data) {
        Ok(res) => Ok(res),
        Err(err) => Err(format!("{:?}", err).to_string()),
    }
}

#[no_mangle]
pub unsafe extern "C" fn decode_invoice(invoice: *const c_char) -> CResult<Invoice> {
    CResult::from_result(parse_invoice(invoice).map(|invoice| Invoice {
        amount_sat: convert_msats_to_sats(invoice.amount_msats()),
        payment_hash: invoice.payment_hash().0,
        expiry_date: (invoice.created_at() + invoice.relative_expiry()).as_secs(),
    }))
}

#[no_mangle]
pub unsafe extern "C" fn check_invoice_is_for_offer(
    invoice: *const c_char,
    offer: *const c_char,
) -> bool {
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
    if let Some(signer) = offer.issuer_signing_pubkey() {
        possible_signers.push(signer);
    }
    possible_signers.contains(&invoice.signing_pubkey())
}

#[no_mangle]
pub extern "C" fn free_c_string(s: *mut c_char) {
    if s.is_null() {
        return;
    }
    unsafe {
        let str = CString::from_raw(s);
        drop(str);
    }
}
