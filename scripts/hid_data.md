## HID Command Log

Turn off gear indicator light: 5aa54803004b000000000000000000000000000000000000

Turn on gear indicator light: 5aa54803014c000000000000000000000000000000000000


Gear 1 Low: 5aa526050014054400000000000000000000000000000000

1405: 1300

Gear 1 Medium: 5aa5260500a406d500000000000000000000000000000000

a406: 1700

Gear 1 High: 5aa52605006c079e00000000000000000000000000000000

6c07: 1900

Gear 2 Low: 5aa526050134086800000000000000000000000000000000

3408: 2100

Gear 2 Medium: 5aa526050160099500000000000000000000000000000000

6009: 2310


Gear 2 High: 5aa52605018c0ac200000000000000000000000000000000

8c0a: 2760

Gear 3 Low: 5aa5260502f00a2700000000000000000000000000000000

f00a: 2800

Gear 3 Medium: 5aa5260502b80bf000000000000000000000000000000000

b80b: 3000

Gear 3 High: 5aa5260502e40c1d00000000000000000000000000000000

e40c: 3300

Gear 4 Low: 5aa5260503ac0de700000000000000000000000000000000

ac0d: 3500

Gear 4 Medium: 5aa5260503740eb000000000000000000000000000000000

740e: 3700

Gear 4 High: 5aa5260503a00fdd00000000000000000000000000000000

a00f: 4000

Gear 1: Silent

Gear 2: Standard

Gear 3: Turbo

Gear 4: Overclock

Power-on auto-start ON:
5aa50c030211000000000000000000000000000000000000

Power-on auto-start OFF:
5aa50c030110000000000000000000000000000000000000

Smart start/stop OFF:
5aa50d030010000000000000000000000000000000000000

Smart start/stop ON (immediate): 5aa50d030111000000000000000000000000000000000000

Smart start/stop ON (delayed):
5aa50d030212000000000000000000000000000000000000




Light brightness:


Apply:
5aa543024500000000000000000000000000000000000000

5aa541024300000000000000000000000000000000000000

0%:
5aa5470d1c00ff00000000000000006f0000000000000000

100%:
5aa543024500000000000000000000000000000000000000


## Fan Speed, Real-time Adjustment

Enter real-time speed change mode:
5aa523022500000000000000000000000000000000000000

Real-time speed setting (examples):
5aa521048d0fc10000000000000000000000000000000000
8d0f: 3981
5aa52104e40c150000000000000000000000000000000000
e40c: 3300
5aa52104a406cf0000000000000000000000000000000000
a406: 1700

Checksum = (byte0 + byte1 + byte2 + byte3 + byte4 + byte5 + 1) & 0xFF



# Received
## Fan Gear Positions
1: 01 5A A5 EF 0B 68 04 05 80 0C 14 05 C7 00 D7 00 00 00 00 00 00 00 00 00 00
2: 01 5A A5 EF 0B 6A 04 05 D0 07 34 08 E4 00 64 00 00 00 00 00 00 00 00 00 00
3: 01 5A A5 EF 0B 6C 04 05 60 09 F0 0A 15 01 E8 00 00 00 00 00 00 00 00 00 00
4: 01 5A A5 EF 0B 6E 04 05 54 0B AC 0D 34 01 BE 00 00 00 00 00 00 00 00 00 00

0: 01 5A A5 EF 0B 68 05 05 C4 09 8D 0F A1 01 77 00 00 00 00 00 00 00 00 00 00

-: 01 5A A5 EF 0B 48 04 05 78 05 14 05 85 00 66 00 00 00 00 00 00 00 00 00 00

Offset | Size | Name               | Value     | Note
---------------------------------------------------------------
0      | 1    | Report ID          | 0x01      | Report type
1-2    | 2    | Magic / Sync       | 0x5AA5    | Sync header, fixed value
3      | 1    | Command / Type     | 0xEF      | Possibly a command code; the primary command code for fan monitoring is 0xEF
4      | 1    | Status / Mode      | 0x0B      | Status byte
5      | 1    | Max Gear & Set Gear      | 0x68      | Split: high nibble represents max gear, low nibble represents set gear. E.g., 0x68 = max gear "Standard" + set gear "Silent" (modes: 2=Standard, 4=Turbo, 6=Overclock; gears: 8=Silent, A=Standard, C=Turbo, E=Overclock)
6      | 1    | Current Mode      | 0x05      | Current mode: 0x04 = Gear operating mode; 0x05 = Auto mode (real-time speed mode)
7      | 1    | Reserved          | 0x05      | Unknown
8-9    | 2    | Fan Real-time RPM      | 0xc409 | Fan speed in RPM, little-endian; 0x09c4 = 2500
9-10  | 2    | Fan Target RPM      | 0x8d0f | Fan target speed in RPM, little-endian; 0x0f8d = 3981
11-24 | 14   | Reserved          | 0x00      | Unknown


Minimum 1000

When max gear is Standard, the speed upper limit is 2760

When max gear is Turbo, the speed upper limit is 3300

When max gear is Overclock, the speed upper limit is 4000
