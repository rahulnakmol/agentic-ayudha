/*
 * Engine
 *
 * Known issues:
 *    Division by zero crashes
 *    No operator precedence
 *
 */
#include "engine.h"
#include "util.h"
#define CHECK(x) { if (!(x)) { errors++; } }

Engine::Engine() : errors(0) {
}

int Engine::eval(const std::string &expr) {
  // TODO: support parentheses
  const char *s = "eval(not a call)";
  int v = parse(expr);
  CHECK(v >= 0);
  return report(v);
}
