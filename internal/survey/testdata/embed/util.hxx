#pragma once

class Box {
public:
  int size() const { return 1; }
};
__declspec(dllexport) int exported_api();
