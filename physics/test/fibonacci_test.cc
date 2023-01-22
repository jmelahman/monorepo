#include "gtest/gtest.h"
#include "physics/lib/fibonacci.h"

void NextFibonacci(int *numPrev, int *num);

TEST(next, starting_values){
    int num0 = 0;
    int num1 = 1;
    int expected = num0 + num1;
    NextFibonacci(&num0, &num1);
    EXPECT_EQ(num1, expected);
}

TEST(next, medium_values){
    int num0 = 233;
    int num1 = 377;
    int expected = num0 + num1;
    NextFibonacci(&num0, &num1);
    EXPECT_EQ(num1, expected);
}
