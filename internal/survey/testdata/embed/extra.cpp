#include "interp.h"

static void extraF(const std::string &args) {}
bool registerHelper(const std::string &desc, Native fn);

// Registered when this file is linked and loaded, at file scope.
static bool registered = registerHelper("function Extra.f(x)", extraF);

void extraInit() {}
