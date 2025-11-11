`circom ex.circom --sym --r1cs --wasm -l /Users/donglieu/1125`


`echo '{"x": "3"}' > input.json`


cd ex_js

node generate_witness.js ex.wasm ../input.json witness.wtns

snarkjs wej witness.wtns && cat witness.json