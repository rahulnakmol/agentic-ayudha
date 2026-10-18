#ifndef INTERP_H
#define INTERP_H
#include <string>

typedef void (*Native)(const std::string &args);

class Interp {
public:
  Interp();
  void addNative(const std::string &desc, Native fn);
  void run(const std::string &code);
  /// Get the value of a variable by dotted path
  std::string getVariable(const std::string &path);
private:
  void step();
};

void registerCore(Interp *in);
void registerMath(Interp *in);
int alpha(); int beta();

class Json {
public:
  Json(Interp *in);
};

#endif
