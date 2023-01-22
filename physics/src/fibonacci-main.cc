/* Calculate the first n Fibonacci numbers */

#include <iostream>
#include "physics/lib/fibonacci.h"
using namespace std;

void NextFibonacci(int *numPrev, int *num);

int main() {
    int numPrev = 0;
    int num = 1;
    cout << numPrev << "\n";
    cout << num << "\n";
    for (int n = 0; n < 10; n++ ) {
        NextFibonacci(&numPrev, &num);
        cout << num << "\n";
    }
    return 0;
}
