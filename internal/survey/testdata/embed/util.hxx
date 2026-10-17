#pragma once

class Box {
public:
  int size() const { return 1; }
};
__declspec(dllexport) int exported_api();
__attribute__((visibility("default"))) int exported_attr();
[[deprecated("old")]] int exported_old();
