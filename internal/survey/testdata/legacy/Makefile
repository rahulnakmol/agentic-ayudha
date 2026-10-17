CXX=g++
SOURCES= \
src/engine.cpp \
src/util.cpp

OBJECTS=$(SOURCES:.cpp=.o)

calc: src/main.o $(OBJECTS)
	$(CXX) -o $@ $^

clean:
	rm -f calc
