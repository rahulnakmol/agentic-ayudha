#include "interp.h"

static void jsonParse(const std::string &args) {}

// Registers when a runner constructs a Json, which no call names.
Json::Json(Interp *in) {
  in->addNative("function JSON.parse(text)", jsonParse);
}
