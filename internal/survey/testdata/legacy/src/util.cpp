#include "util.h"
#include <stdio.h>
#include <stdlib.h>

int parse(const std::string &s) {
  if (getenv("CALC_DEBUG")) {
    printf("parse %s\n", s.c_str());
  }
  return atoi(s.c_str());
}

static int report(int v)
{
  printf("%d\n", v);
  return v;
}
