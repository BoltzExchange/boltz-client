extern crate lightning;
extern crate bech32;

use std::convert::TryFrom;
use std::ffi::{c_char, CStr};
use lightning::offers::offer::Amount;
use bech32::FromBase32;

const BECH32_BOLT12_INVOICE_HRP: &str = "lni";

#[no_mangle]
pub unsafe extern "C" fn decode_offer(offer: *const c_char) -> *mut Offer {
    let raw = CStr::from_ptr(offer);
    let offer = match raw.to_str() {
        Ok(res) => res,
        Err(_) => return std::ptr::null_mut(),
    };
    let offer = match offer.parse::<lightning::offers::offer::Offer>() {
        Ok(res) => res,
        Err(_) => return std::ptr::null_mut(),
    };
    Box::into_raw(Box::new(Offer {
        amount:
        match offer.amount() {
            Some(amount) => match amount {
                Amount::Bitcoin { amount_msats } => amount_msats / 1000,
                Amount::Currency { .. } => 0,
            },
            None => 0,
        },
    }))
}

#[no_mangle]
pub unsafe extern "C" fn decode_invoice(invoice: *const c_char) -> *mut Invoice {
    let raw = CStr::from_ptr(invoice);
    let invoice = match raw.to_str() {
        Ok(res) => res,
        Err(_) => return std::ptr::null_mut(),
    };
    let (hrp, data) = match bech32::decode_without_checksum(invoice) {
        Ok(res) => res,
        Err(_) => return std::ptr::null_mut(),
    };
    if hrp.as_str() != BECH32_BOLT12_INVOICE_HRP {
        return std::ptr::null_mut();
    }

    let data = match Vec::<u8>::from_base32(&data) {
        Ok(res) => res,
        Err(_) => return std::ptr::null_mut(),
    };
    match lightning::offers::invoice::Bolt12Invoice::try_from(data) {
        Ok(invoice) => Box::into_raw(Box::new(Invoice {
            amount: invoice.amount_msats() / 1000,
            payment_hash: invoice.payment_hash().0,
            expiry_date: invoice.absolute_expiry().and_then(|d| Some(d.as_secs())).unwrap_or(0)
        })),
        Err(_) => std::ptr::null_mut(),
    }
}

#[repr(C)]
pub struct Offer {
    pub amount: u64,
}

#[repr(C)]
pub struct Invoice {
    pub amount: u64,
    pub payment_hash: [u8; 32],
    pub expiry_date : u64,
}