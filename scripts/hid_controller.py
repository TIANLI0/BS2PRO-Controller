import hid
import time
from typing import Optional, List, Tuple


class BS2PROHIDController:
    """BS2PRO HID Controller"""

    def __init__(self):
        self.device = None
        self.vendor_id = None
        self.product_id = None

    def find_bs2pro_devices(self) -> List[Tuple[int, int, str]]:
        """Find all possible BS2PRO HID devices"""
        devices = []

        print("Searching for BS2PRO HID devices...")

        # Add known BS2PRO device information
        known_bs2pro_devices = [
            (0x137D7, 0x1002, "FlyDigi BS2PRO"),
        ]

        # Enumerate all devices and find matches
        for device_info in hid.enumerate():
            vendor_id = device_info.get("vendor_id", 0)
            product_id = device_info.get("product_id", 0)
            manufacturer = device_info.get("manufacturer_string", "")
            product_name = device_info.get("product_string", "")

            # First check known devices
            for known_vid, known_pid, known_name in known_bs2pro_devices:
                if vendor_id == known_vid and product_id == known_pid:
                    devices.append((vendor_id, product_id, known_name))
                    print(f"Found known BS2PRO device:")
                    print(f"  Vendor ID: 0x{vendor_id:04X}")
                    print(f"  Product ID: 0x{product_id:04X}")
                    print(f"  Manufacturer: {manufacturer}")
                    print(f"  Product Name: {product_name}")
                    print("-" * 40)
                    continue

            # Extended search criteria
            search_terms = ["BS2PRO", "FLYDIGI", "FLY", "CONTROLLER", "GAMEPAD"]
            is_match = False

            for term in search_terms:
                if (
                    term in str(manufacturer).upper()
                    or term in str(product_name).upper()
                ):
                    is_match = True
                    break

            # Also check common gamepad vendor IDs
            common_gamepad_vendors = [
                0x2DC8,  # 8BitDo
                0x045E,  # Microsoft
                0x054C,  # Sony
                0x057E,  # Nintendo
                0x0F0D,  # Hori
                0x28DE,  # Valve
                0x137D7,  # FlyDigi
            ]

            if vendor_id in common_gamepad_vendors:
                is_match = True

            if is_match and (vendor_id, product_id) not in [
                (v, p) for v, p, _ in devices
            ]:
                devices.append(
                    (
                        vendor_id,
                        product_id,
                        product_name or f"Unknown-{vendor_id:04X}:{product_id:04X}",
                    )
                )
                print(f"Found potential device:")
                print(f"  Vendor ID: 0x{vendor_id:04X}")
                print(f"  Product ID: 0x{product_id:04X}")
                print(f"  Manufacturer: {manufacturer}")
                print(f"  Product Name: {product_name}")
                print("-" * 40)

        return devices

    def connect(
        self, vendor_id: Optional[int] = None, product_id: Optional[int] = None
    ) -> bool:
        """Connect to device"""
        try:
            if vendor_id is None or product_id is None:
                # Find potential BS2PRO devices
                devices = self.find_bs2pro_devices()
                if not devices:
                    print("No matching HID devices found")
                    print("\nTip: Manually specify vendor ID and product ID")
                    print("   Example: controller.connect(0x1234, 0x5678)")
                    return False

                # Try connecting to each found device
                for vid, pid, name in devices:
                    print(f"Attempting to connect to: {name} (0x{vid:04X}:0x{pid:04X})")
                    if self._try_connect(vid, pid):
                        return True

                print("All potential device connections failed")
                return False
            else:
                return self._try_connect(vendor_id, product_id)

        except Exception as e:
            print(f"Connection error: {e}")
            return False

    def _try_connect(self, vendor_id: int, product_id: int) -> bool:
        """Try to connect to a specified device"""
        try:
            self.device = hid.device()
            self.device.open(vendor_id, product_id)
            self.vendor_id = vendor_id
            self.product_id = product_id

            # Get device information
            manufacturer = self.device.get_manufacturer_string() or "Unknown"
            product = self.device.get_product_string() or "Unknown"

            print(f"Connected successfully!")
            print(f"  Manufacturer: {manufacturer}")
            print(f"  Product: {product}")
            print(f"  Vendor ID: 0x{vendor_id:04X}")
            print(f"  Product ID: 0x{product_id:04X}")

            return True

        except Exception as e:
            print(f"Failed to connect to 0x{vendor_id:04X}:0x{product_id:04X}: {e}")
            if self.device:
                try:
                    self.device.close()
                except:
                    pass
                self.device = None
            return False

    def send_feature_report(self, report_id: int, data: bytes) -> bool:
        """Send feature report"""
        try:
            if not self.device:
                print("Device not connected")
                return False

            # Construct report (report ID + data)
            report = bytes([report_id]) + data
            result = self.device.send_feature_report(report)
            print(f"Feature report sent successfully: report_id={report_id}, length={result}")
            return True

        except Exception as e:
            print(f"Failed to send feature report: {e}")
            return False

    def get_feature_report(self, report_id: int, length: int = 64) -> Optional[bytes]:
        """Get feature report"""
        try:
            if not self.device:
                print("Device not connected")
                return None

            report = self.device.get_feature_report(report_id, length)
            print(f"Feature report received: report_id={report_id}, data={bytes(report).hex()}")
            return bytes(report)

        except Exception as e:
            print(f"Failed to get feature report: {e}")
            return None

    def send_output_report(self, data: bytes) -> bool:
        """Send output report"""
        try:
            if not self.device:
                print("Device not connected")
                return False

            result = self.device.write(data)
            print(f"Output report sent successfully: length={result}")
            return True

        except Exception as e:
            print(f"Failed to send output report: {e}")
            return False

    def read_input_report(self, timeout: int = 1000) -> Optional[bytes]:
        """Read input report"""
        try:
            if not self.device:
                print("Device not connected")
                return None

            # Set non-blocking mode
            self.device.set_nonblocking(True)
            data = self.device.read(64, timeout)

            if data:
                print(f"Input report received: data={bytes(data).hex()}")
                return bytes(data)
            else:
                print("No input report received (timeout or no data)")
                return None

        except Exception as e:
            print(f"Failed to read input report: {e}")
            return None

    def send_hex_command(
        self, hex_string: str, report_id: int = 0x02, padding_length: int = 23
    ) -> bool:
        """Send hex command

        Args:
            hex_string: Hex string (e.g., "5aa5410243")
            report_id: Report ID
            padding_length: Total length, padded with zeros if insufficient
        """
        try:
            # Remove spaces and convert to bytes
            hex_string = hex_string.replace(" ", "").replace("0x", "")
            payload = bytes.fromhex(hex_string)

            # Calculate padding length needed (total length - report ID (1 byte) - payload length)
            padding_needed = padding_length - 1 - len(payload)
            if padding_needed < 0:
                print(
                    f"Warning: Command length ({len(payload)}) exceeds maximum allowed length ({padding_length-1})"
                )
                padding_needed = 0

            padding = bytes(padding_needed)
            command = bytes([report_id]) + payload + padding

            print(f"Sending command: {hex_string} (total length: {len(command)})")
            return self.send_output_report(command)

        except ValueError as e:
            print(f"Hex format error: {e}")
            return False
        except Exception as e:
            print(f"Failed to send command: {e}")
            return False

    def send_multiple_commands(self, commands: List[str], delay: float = 0.1) -> int:
        """Send multiple commands

        Args:
            commands: List of commands
            delay: Delay between commands (seconds)

        Returns:
            Number of successfully sent commands
        """
        success_count = 0

        for i, cmd in enumerate(commands):
            cmd = cmd.strip()
            if not cmd:  # Skip empty lines
                continue

            print(f"\nCommand {i+1}/{len(commands)}: {cmd}")
            if self.send_hex_command(cmd):
                success_count += 1

            # Delay between commands
            if delay > 0 and i < len(commands) - 1:
                time.sleep(delay)

        print(
            f"\nDone! Successfully sent {success_count}/{len([c for c in commands if c.strip()])} commands"
        )
        return success_count

    def calculate_checksum(self, rpm: int) -> int:
        """Calculate checksum for speed command"""
        # Construct first 6 bytes: 5aa52104 + speed in little-endian bytes
        speed_bytes = rpm.to_bytes(2, "little")

        # First 6 bytes
        byte0 = 0x5A
        byte1 = 0xA5
        byte2 = 0x21
        byte3 = 0x04
        byte4 = speed_bytes[0]  # Speed low byte
        byte5 = speed_bytes[1]  # Speed high byte

        # Checksum byte = (sum of first 6 bytes + 1) & 0xFF
        checksum = (byte0 + byte1 + byte2 + byte3 + byte4 + byte5 + 1) & 0xFF
        return checksum

    def enter_realtime_speed_mode(self) -> bool:
        """Enter real-time speed change mode"""
        print("Entering real-time speed change mode...")
        return self.send_hex_command("5aa523022500000000000000000000000000000000000000")

    def set_fan_speed(self, rpm: int) -> bool:
        """Set fan speed

        Args:
            rpm: Speed value (recommended range: 1000-4000)
        """
        if not 0 <= rpm <= 65535:
            print(f"Speed value out of range: {rpm} (valid range: 0-65535)")
            return False

        # Convert speed to little-endian bytes
        speed_bytes = rpm.to_bytes(2, "little")

        # Calculate checksum
        checksum = self.calculate_checksum(rpm)

        # Construct full command
        command = (
            f"5aa52104{speed_bytes.hex()}{checksum:02x}00000000000000000000000000000000"
        )

        print(f"Setting fan speed: {rpm} RPM")
        print(f"Command: {command}")

        return self.send_hex_command(command)

    def set_gear_position(self, gear: int, position: int) -> bool:
        """Set gear position

        Args:
            gear: Gear level (1-4)
            position: Position within gear (1-3)
        """
        gear_positions = {
            (1, 1): "5aa526050014054400000000000000000000000000000000",
            (1, 2): "5aa5260500a406d500000000000000000000000000000000",
            (1, 3): "5aa52605006c079e00000000000000000000000000000000",
            (2, 1): "5aa526050134086800000000000000000000000000000000",
            (2, 2): "5aa526050160099500000000000000000000000000000000",
            (2, 3): "5aa52605018c0ac200000000000000000000000000000000",
            (3, 1): "5aa5260502f00a2700000000000000000000000000000000",
            (3, 2): "5aa5260502b80bf000000000000000000000000000000000",
            (3, 3): "5aa5260502e40c1d00000000000000000000000000000000",
            (4, 1): "5aa5260503ac0de700000000000000000000000000000000",
            (4, 2): "5aa5260503740eb000000000000000000000000000000000",
            (4, 3): "5aa5260503a00fdd00000000000000000000000000000000",
        }

        if (gear, position) not in gear_positions:
            print(f"Invalid gear setting: gear {gear} position {position}")
            print("Valid range: gears 1-4, each with positions 1-3")
            return False

        command = gear_positions[(gear, position)]
        print(f"Setting gear: gear {gear} position {position}")
        return self.send_hex_command(command)

    def set_gear_light(self, enabled: bool) -> bool:
        """Set gear indicator light on/off"""
        if enabled:
            command = "5aa54803014c000000000000000000000000000000000000"
            print("Turning on gear indicator light")
        else:
            command = "5aa54803004b000000000000000000000000000000000000"
            print("Turning off gear indicator light")
        return self.send_hex_command(command)

    def set_power_on_start(self, enabled: bool) -> bool:
        """Set power-on auto-start"""
        if enabled:
            command = "5aa50c030211000000000000000000000000000000000000"
            print("Enabling power-on auto-start")
        else:
            command = "5aa50c030110000000000000000000000000000000000000"
            print("Disabling power-on auto-start")
        return self.send_hex_command(command)

    def set_smart_start_stop(self, mode: str) -> bool:
        """Set smart start/stop

        Args:
            mode: 'off', 'immediate', 'delayed'
        """
        commands = {
            "off": "5aa50d030010000000000000000000000000000000000000",
            "immediate": "5aa50d030111000000000000000000000000000000000000",
            "delayed": "5aa50d030212000000000000000000000000000000000000",
        }

        if mode not in commands:
            print(f"Invalid smart start/stop mode: {mode}")
            print("Valid modes: 'off', 'immediate', 'delayed'")
            return False

        print(f"Setting smart start/stop: {mode}")
        return self.send_hex_command(commands[mode])

    def set_brightness(self, percentage: int) -> bool:
        """Set light brightness

        Args:
            percentage: Brightness percentage (0-100)
        """
        if percentage == 0:
            command = "5aa5470d1c00ff00000000000000006f0000000000000000"
            print("Setting brightness: 0%")
        elif percentage == 100:
            command = "5aa543024500000000000000000000000000000000000000"
            print("Setting brightness: 100%")
        else:
            print(f"Currently only 0% and 100% brightness settings are supported")
            return False

        return self.send_hex_command(command)

    def disconnect(self):
        """Disconnect"""
        if self.device:
            try:
                self.device.close()
            except:
                pass
            self.device = None
            print("Device disconnected")


def test_bs2pro_with_commands():
    """Test BS2PRO device and send custom commands"""
    controller = BS2PROHIDController()

    print("Connecting to BS2PRO device...")
    print("=" * 50)

    # Use known vendor ID and product ID directly
    if not controller.connect(0x137D7, 0x1002):
        print("Unable to connect to BS2PRO device")
        return False

    print("\nStarting to send commands...")
    print("-" * 30)

    # Test command list - one command per line
    test_commands = [
        "5aa54803014b0",
    ]

    # Send multiple commands
    controller.send_multiple_commands(test_commands, delay=0.2)

    # Test reading input
    print("\nListening for input data...")
    print("  (Press and hold controller buttons to see data...)")
    for i in range(3):
        response = controller.read_input_report(1000)
        if response and any(response):
            print(f"  Input {i+1}: {response[:16].hex()}...")
            break
        else:
            print(f"  Input {i+1}: No data")

    print("\nTest completed!")
    controller.disconnect()
    return True


def interactive_command_mode():
    """Interactive command mode"""
    controller = BS2PROHIDController()

    print("Interactive Command Mode")
    print("=" * 50)

    if not controller.connect(0x137D7, 0x1002):
        print("Unable to connect to BS2PRO device")
        return

    print("\nUsage:")
    print("  - Enter hex commands (e.g., 5aa5410243)")
    print("  - Separate multiple lines with Enter, empty line to finish")
    print("  - Enter 'quit' or 'exit' to quit")
    print("  - Enter 'listen' to listen for input data")
    print("  - Enter 'speed <rpm>' to set speed (e.g., speed 2000)")
    print("  - Enter 'gear <level> <position>' to set gear (e.g., gear 2 3)")
    print("-" * 30)

    while True:
        try:
            print("\nEnter command:")
            line = input().strip()

            if not line:
                continue

            if line.lower() in ["quit", "exit"]:
                break

            if line.lower() == "listen":
                print("Listen mode (5 seconds)...")
                for i in range(5):
                    response = controller.read_input_report(1000)
                    if response and any(response):
                        print(f"  Input: {response[:16].hex()}...")
                continue

            if line.lower().startswith("speed "):
                try:
                    rpm = int(line.split()[1])
                    controller.enter_realtime_speed_mode()
                    time.sleep(0.1)
                    controller.set_fan_speed(rpm)
                except (IndexError, ValueError):
                    print("Usage: speed <rpm_value>")
                continue

            if line.lower().startswith("gear "):
                try:
                    parts = line.split()
                    gear = int(parts[1])
                    position = int(parts[2])
                    controller.set_gear_position(gear, position)
                except (IndexError, ValueError):
                    print("Usage: gear <level> <position>")
                continue

            # Regular hex command
            controller.send_hex_command(line)

        except KeyboardInterrupt:
            print("\n\nUser interrupted, exiting...")
            break
        except Exception as e:
            print(f"Error: {e}")

    controller.disconnect()


if __name__ == "__main__":
    print("Select mode:")
    print("1. Test preset commands")
    print("2. Interactive command mode")

    choice = input("Please choose (1/2): ").strip()

    if choice == "2":
        interactive_command_mode()
    else:
        test_bs2pro_with_commands()
