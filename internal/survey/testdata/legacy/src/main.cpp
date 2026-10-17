#include "engine.h"
#include <stdio.h>

int main(int argc, char **argv) {
  Engine e;
  if (argc < 2) {
    fprintf(stderr, "usage: calc EXPR\n");
    return 2;
  }
  return e.eval(argv[1]) == 0 ? 1 : 0;
}
