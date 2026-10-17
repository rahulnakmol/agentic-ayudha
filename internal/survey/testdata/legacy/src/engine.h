#ifndef ENGINE_H
#define ENGINE_H
#include <string>

/* Evaluates arithmetic. */
class Engine {
public:
  Engine();
  int eval(const std::string &expr);
  bool ok() const { return errors == 0; }
private:
  int errors;
};

#endif
