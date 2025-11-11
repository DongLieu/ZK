pragma circom 2.2.3;

// assert that a*b === c*d
template Example() {
  signal input a;
  signal input b;
  signal input c;
  signal input d;

  signal s;

  s <== a * b;
  d === s * c;
}

component main {public [c, d]} = Example();