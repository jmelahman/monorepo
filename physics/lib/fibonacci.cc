/* Calculate Fibonacci numbers */

void NextFibonacci(int *numPrev, int *num) {
  int numOld = *num;
  *num = *numPrev + *num;
  *numPrev = numOld;
}
