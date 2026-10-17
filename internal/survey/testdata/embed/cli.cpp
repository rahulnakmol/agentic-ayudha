#include "interp.h"

int main(int argc, char **argv) {
  Interp in;
  registerCore(&in);
  if (argc > 1) in.run(argv[1]);
  return 0;
}
