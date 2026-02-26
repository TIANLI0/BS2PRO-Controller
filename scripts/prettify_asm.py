#!/usr/bin/env python3

import argparse
import re
from dataclasses import dataclass
from pathlib import Path


LINE_RE = re.compile(r"^(0x[0-9a-fA-F]+):\s+([0-9a-fA-F]+)\s+([^\s]+)(?:\s+(.*))?$")

ABS_HEX_RE = re.compile(r"^0x[0-9a-fA-F]+$")
REL_RE = re.compile(r"^-?(?:0x[0-9a-fA-F]+|\d+)$")

CONTROL_FLOW_MNEMONICS = {
    "j",
    "jal",
    "jalr",
    "beq",
    "bne",
    "blt",
    "bge",
    "bltu",
    "bgeu",
    "c.j",
    "c.jal",
    "c.jr",
    "c.jalr",
    "c.beqz",
    "c.bnez",
}


@dataclass
class Instruction:
    address: int
    raw_bytes: str
    mnemonic: str
    op_str: str
    original: str


def parse_int(token: str) -> int:
    return int(token, 0)


def parse_asm_lines(lines: list[str]) -> tuple[list[str], list[Instruction], list[str]]:
    header: list[str] = []
    instructions: list[Instruction] = []
    trailer: list[str] = []

    seen_instruction = False
    for line in lines:
        stripped = line.rstrip("\n")
        match = LINE_RE.match(stripped)
        if match:
            seen_instruction = True
            address_text, raw_bytes, mnemonic, op_str = match.groups()
            instructions.append(
                Instruction(
                    address=parse_int(address_text),
                    raw_bytes=raw_bytes,
                    mnemonic=mnemonic,
                    op_str=(op_str or "").strip(),
                    original=stripped,
                )
            )
            continue

        if not seen_instruction:
            header.append(stripped)
        else:
            trailer.append(stripped)

    return header, instructions, trailer


def infer_target(inst: Instruction) -> int | None:
    if inst.mnemonic not in CONTROL_FLOW_MNEMONICS:
        return None
    if not inst.op_str:
        return None

    tokens = [part.strip() for part in inst.op_str.split(",")]
    last = tokens[-1]

    if ABS_HEX_RE.match(last):
        return parse_int(last)

    if REL_RE.match(last):
        offset = parse_int(last)
        return inst.address + offset

    return None


def rewrite_op_str(inst: Instruction, labels: dict[int, str]) -> str:
    if not inst.op_str:
        return inst.op_str

    tokens = [part.strip() for part in inst.op_str.split(",")]
    last = tokens[-1]
    target = infer_target(inst)
    if target is None:
        return inst.op_str

    label = labels.get(target)
    if not label:
        return inst.op_str

    tokens[-1] = label
    return ", ".join(tokens)


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Prettify disassembled ASM with labels"
    )
    parser.add_argument("--input", required=True, help="Input ASM path")
    parser.add_argument("--output", required=True, help="Output pretty ASM path")
    args = parser.parse_args()

    input_path = Path(args.input)
    output_path = Path(args.output)

    if not input_path.exists():
        raise FileNotFoundError(f"Input ASM not found: {input_path}")

    lines = input_path.read_text(encoding="utf-8", errors="replace").splitlines()
    header, instructions, trailer = parse_asm_lines(lines)

    addresses = {inst.address for inst in instructions}
    max_address = max(addresses) if addresses else 0

    targets: set[int] = set()
    for inst in instructions:
        target = infer_target(inst)
        if target is None:
            continue
        if target < 0 or target > max_address:
            continue
        if target not in addresses:
            continue
        targets.add(target)

    labels = {address: f"loc_{address:08x}" for address in sorted(targets)}

    output_path.parent.mkdir(parents=True, exist_ok=True)
    with output_path.open("w", encoding="utf-8") as output_file:
        for line in header:
            output_file.write(line + "\n")

        output_file.write("\n; ===== Pretty View =====\n")
        output_file.write(f"; labels={len(labels)}\n\n")

        for inst in instructions:
            label = labels.get(inst.address)
            if label:
                output_file.write(f"\n{label}:\n")

            op_str = rewrite_op_str(inst, labels)
            output_file.write(
                f"0x{inst.address:08x}:\t{inst.raw_bytes:<12}\t{inst.mnemonic}\t{op_str}\n"
            )

        for line in trailer:
            output_file.write(line + "\n")

    print(f"Pretty ASM generated: {output_path} (labels={len(labels)})")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
