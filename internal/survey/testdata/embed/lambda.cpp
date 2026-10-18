#include "interp.h"

bool registerHelper(const std::string &desc, Native fn);

// Linked into run_tests; the callback is an inline lambda.
static bool registered = registerHelper("function Lam.go()", [](const std::string &args) {});
