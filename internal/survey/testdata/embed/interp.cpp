#include "interp.h"
#include <cstdio>
#include <cstring>

Interp::Interp() {}

void Interp::addNative(const std::string &desc, Native fn) {
  printf("native %s\n", desc.c_str());
}

void Interp::run(const std::string &code) {
  if (strcmp("quit", code.c_str()) == 0) return;
  step();
}

std::string Interp::getVariable(const std::string &path) { return path; }

void Interp::step() {}

static void corePrint(const std::string &args) { printf("%s\n", args.c_str()); }
static void mathSqrt(const std::string &args) { printf("sqrt %s\n", args.c_str()); }

void registerCore(Interp *in) {
  in->addNative("function print(text)", corePrint);
  // in->addNative("function Math.cbrt(x)", mathSqrt);
}

void registerMath(Interp *in) {
  in->addNative("function Math.sqrt(x)", mathSqrt);
}
