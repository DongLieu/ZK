import { createRequire } from "module";
import { readFileSync, writeFileSync } from "fs";

const require = createRequire(import.meta.url);
const witnessCalculatorBuilder = require("./witness_calculator.cjs");

async function main() {
    if (process.argv.length !== 5) {
        console.log("Usage: node generate_witness.js <file.wasm> <input.json> <output.wtns>");
        process.exit(1);
    }

    const wasmPath = process.argv[2];
    const inputPath = process.argv[3];
    const outputPath = process.argv[4];

    const input = JSON.parse(readFileSync(inputPath, "utf8"));
    const buffer = readFileSync(wasmPath);

    const witnessCalculator = await witnessCalculatorBuilder(buffer);
    const witness = await witnessCalculator.calculateWTNSBin(input, 0);
    writeFileSync(outputPath, witness);
}

main().catch((err) => {
    console.error(err);
    process.exit(1);
});
