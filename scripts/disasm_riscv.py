#!/usr/bin/env python3

import argparse
from pathlib import Path

from capstone import Cs, CS_ARCH_RISCV, CS_MODE_LITTLE_ENDIAN, CS_MODE_RISCV32


def parse_int(value: str) -> int:
    return int(value, 0)


def create_disassembler(enable_compressed: bool) -> Cs:
    mode = CS_MODE_RISCV32 | CS_MODE_LITTLE_ENDIAN
    if enable_compressed:
        try:
            from capstone import CS_MODE_RISCVC

            mode |= CS_MODE_RISCVC
        except ImportError:
            pass

    disassembler = Cs(CS_ARCH_RISCV, mode)
    disassembler.detail = False
    disassembler.skipdata = True
    return disassembler


def main() -> int:
    parser = argparse.ArgumentParser(description="Disassemble RISC-V BIN to ASM text")
    parser.add_argument("--input", required=True, help="Input BIN path")
    parser.add_argument("--output", required=True, help="Output ASM path")
    parser.add_argument(
        "--base", default="0x0", type=parse_int, help="Load base address"
    )
    parser.add_argument(
        "--no-compressed",
        action="store_true",
        help="Disable RISC-V compressed extension decoding",
    )
    args = parser.parse_args()

    input_path = Path(args.input)
    output_path = Path(args.output)

    if not input_path.exists():
        raise FileNotFoundError(f"Input BIN not found: {input_path}")

    data = input_path.read_bytes()
    disassembler = create_disassembler(enable_compressed=not args.no_compressed)

    output_path.parent.mkdir(parents=True, exist_ok=True)
    with output_path.open("w", encoding="utf-8") as output_file:
        output_file.write(f"; input={input_path}\n")
        output_file.write(f"; base=0x{args.base:08x}\n")
        output_file.write(f"; size={len(data)} bytes\n\n")

        count = 0
        for instruction in disassembler.disasm(data, args.base):
            count += 1
            output_file.write(
                f"0x{instruction.address:08x}:\t"
                f"{instruction.bytes.hex():<12}\t"
                f"{instruction.mnemonic}\t{instruction.op_str}\n"
            )

        output_file.write(f"\n; instructions={count}\n")

    print(f"ASM generated: {output_path} (instructions={count})")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
