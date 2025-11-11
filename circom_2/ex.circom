pragma circom 2.2.3;

include "circomlib/circuits/comparators.circom";
include "circomlib/circuits/gates.circom";

template DisjointExample1() {
  signal input x;

  signal indicator1;
  signal indicator2;

  indicator1 <== LessThan(252)([x, 5]);
  indicator2 <== GreaterThan(252)([x, 17]);

  component or = OR();
  or.a <== indicator1;
  or.b <== indicator2;

  or.out === 1;
}

component main = DisjointExample1();
