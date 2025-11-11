### To see how the witness is structured:

1 Save the file above as: `ex.circom`
2 Compile it with `circom ex.circom --sym --r1cs --wasm`

3 Create the input.json: `echo '{"a": "3", "b": "4", "c":"2", "d":"24"}' > input.json`
4 `cd ex1_js`
5 Compute the witness: `node generate_witness.js ex1.wasm ../input.json witness.wtns`
6 Convert the witness to json and cat it: `snarkjs wej witness.wtns && cat witness.json`


### We should get the following result. Note that this matches the values we supplied for input.json:

```
[
 "1", // constant
 "2", // c (public signal)
 "24", // d (public signal)
 "3", // a
 "4", // b
 "12" // s
]

```