#include <stdarg.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>

typedef struct Offer {
  uint64_t min_amount_sat;
} Offer;

typedef struct CResult_Offer {
  const struct Offer *result;
  const char *error;
} CResult_Offer;

typedef struct Invoice {
  uint64_t amount_sat;
  uint8_t payment_hash[32];
  uint64_t expiry_date;
} Invoice;

typedef struct CResult_Invoice {
  const struct Invoice *result;
  const char *error;
} CResult_Invoice;

struct CResult_Offer decode_offer(const char *offer);

struct CResult_Invoice decode_invoice(const char *invoice);

bool check_invoice_is_for_offer(const char *invoice, const char *offer);

void free_c_string(char *s);
