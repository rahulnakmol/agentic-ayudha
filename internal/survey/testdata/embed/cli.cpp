#include "interp.h"

int main(int argc, char **argv) {
  Interp in;
  Json *json = 0; // a pointer constructs nothing
  registerCore(&in);
  if (argc > 1) in.run(argv[1]);
  return 0;
}
