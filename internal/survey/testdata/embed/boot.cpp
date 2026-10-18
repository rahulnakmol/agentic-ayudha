#include "interp.h"

bool registerHelper(const std::string &desc, Native fn);
static void bootGo(const std::string &args) {}

// Linked into run_tests by CMakeLists.txt; nothing calls into this file.
static bool registered = registerHelper("function Boot.go()", bootGo);
