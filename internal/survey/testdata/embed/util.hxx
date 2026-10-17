#pragma once

class Box {
public:
  int size() const { return 1; }
};
__declspec(dllexport) int exported_api();
__attribute__((visibility("default"))) int exported_attr();
[[deprecated("old")]] int exported_old();
class __declspec(dllexport) Exported {
public:
  void run();
};
inline int twice(int x) { return 2 * x; }
void over(int a); void over(double b); void over(char c);
