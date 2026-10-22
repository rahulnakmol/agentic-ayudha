project (calc)
# The engine's sources.
set (ENGINE_SOURCES src/engine.cpp src/util.cpp)
add_library(engine STATIC ${ENGINE_SOURCES})
add_executable(calc src/main.cpp ${ENGINE_SOURCES})
find_package(Threads)
