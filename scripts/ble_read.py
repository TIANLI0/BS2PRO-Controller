#!/usr/bin/env python3
"""
FlyDigi BS2PRO Bluetooth Device Data Reading Script
Monitors real-time RPM and target RPM of the device
"""

import asyncio
import struct
import signal
import sys
from typing import Optional
from bleak import BleakClient, BleakScanner
from bleak.backends.device import BLEDevice

# Bluetooth vendor prefix mapping
VENDOR_PREFIXES = {
    "e5:66:e5": "NanjingQinhe",  # Based on packet analysis
    "00:00:00": "Unknown",
    # More vendor prefixes can be added
}

# Device configuration
TARGET_DEVICE_NAME = "FlyDigi BS2PRO"
# Target characteristic UUID - custom characteristic found during device discovery
TARGET_CHARACTERISTIC_UUID = "0000fff1-0000-1000-8000-00805f9b34fb"
SCAN_TIMEOUT = 5.0  # Scan timeout (seconds)


class BS2PROMonitor:
    def __init__(self):
        self.client: Optional[BleakClient] = None
        self.device: Optional[BLEDevice] = None
        self.running = False

    def get_vendor_from_mac(self, mac_address: str) -> str:
        """Get vendor info from MAC address prefix"""
        mac_prefix = mac_address.lower()[:8]  # Take first 3 bytes
        return VENDOR_PREFIXES.get(mac_prefix, "Unknown")

    def parse_speed_data(self, data: bytearray) -> tuple:
        """
        Parse speed data
        Based on packet format: 5aa5ef0b4a0705e40ce40cfb002b00000000000000000000
        Bytes 8-9: Target speed (big-endian uint16)
        Bytes 10-11: Actual speed (big-endian uint16)
        """
        if len(data) < 12:
            print(f"Packet length insufficient: {len(data)} bytes (need at least 12 bytes)")
            return None, None

        print(f"Raw packet: {data.hex()}")

        try:
            # Extract target speed (bytes 8-9, big-endian)
            target_speed_bytes = data[8:10]
            target_speed = struct.unpack(">H", target_speed_bytes)[0]
            print(f"Target speed bytes [8-9]: {target_speed_bytes.hex()} -> {target_speed}")

            # Extract actual speed (bytes 10-11, big-endian)
            actual_speed_bytes = data[10:12]
            actual_speed = struct.unpack(">H", actual_speed_bytes)[0]
            print(f"Actual speed bytes [10-11]: {actual_speed_bytes.hex()} -> {actual_speed}")

            return target_speed, actual_speed
        except struct.error as e:
            print(f"Data parsing error: {e}")
            return None, None

    def notification_handler(self, characteristic, data: bytearray):
        """Handle received notification data"""
        # Get characteristic handle
        handle = characteristic.handle if hasattr(characteristic, "handle") else "Unknown"
        uuid = (
            str(characteristic.uuid).lower()
            if hasattr(characteristic, "uuid")
            else "Unknown"
        )

        print("\n=== Notification Received ===")
        print(f"Characteristic UUID: {uuid}")
        print(f"Handle: 0x{handle:04x}")
        print(f"Data length: {len(data)} bytes")
        print(f"Raw data: {data.hex()}")

        # Try to parse speed data (for all notification characteristics)
        target_speed, actual_speed = self.parse_speed_data(data)
        if target_speed is not None and actual_speed is not None:
            print(f"Target RPM: {target_speed} RPM")
            print(f"Actual RPM: {actual_speed} RPM")
            print(f"RPM Difference: {actual_speed - target_speed} RPM")
            print("-" * 50)
        else:
            print("Unable to parse as speed data")
            print("-" * 50)

    async def scan_devices(self) -> Optional[BLEDevice]:
        """Scan for Bluetooth devices"""
        print(f"Scanning for Bluetooth devices ({SCAN_TIMEOUT} seconds)...")

        # Get discovered devices
        devices = await BleakScanner.discover(timeout=SCAN_TIMEOUT)

        print(f"\nDiscovered {len(devices)} Bluetooth devices:")
        print("-" * 60)

        target_device = None

        for device in devices:
            vendor = self.get_vendor_from_mac(device.address)
            device_name = device.name or "Unknown Device"

            # Display device information
            print(f"Device Name: {device_name}")
            print(f"MAC Address: {device.address}")
            print(f"Vendor: {vendor}")
            # RSSI may not be available on all platforms
            try:
                print(f"RSSI: {getattr(device, 'rssi', 'Unknown')} dBm")
            except Exception:
                print("RSSI: Unknown")

            # Check if this is the target device
            if device.name == TARGET_DEVICE_NAME:
                target_device = device
                print("*** This is the target device ***")

            print("-" * 60)

        if target_device:
            print(f"Target device found: {target_device.name} ({target_device.address})")
        else:
            print(f"Target device not found: {TARGET_DEVICE_NAME}")

        return target_device

    async def connect_and_monitor(self, device: BLEDevice):
        """Connect to device and start monitoring"""
        print(f"Connecting to device: {device.name} ({device.address})")

        try:
            async with BleakClient(device) as client:
                self.client = client
                print(f"Successfully connected to {device.name}")

                # Get device service information
                services = client.services
                service_count = (
                    len(services.services) if hasattr(services, "services") else 0
                )
                print(f"Device service count: {service_count}")

                # Find all notification characteristics
                notification_chars = []
                target_char = None

                for service in services:
                    print(f"\nService: {service.uuid}")
                    for char in service.characteristics:
                        print(
                            f"  Characteristic: {char.uuid} (Handle: 0x{char.handle:04x}) Properties: {char.properties}"
                        )
                        if "notify" in char.properties:
                            print(
                                f"  *** Found notification characteristic: {char.uuid} (Handle: 0x{char.handle:04x}) ***"
                            )
                            notification_chars.append(char)
                            # Prefer the target UUID characteristic
                            if (
                                str(char.uuid).lower()
                                == TARGET_CHARACTERISTIC_UUID.lower()
                            ):
                                target_char = char
                                print("  *** This is the target notification characteristic ***")

                if notification_chars:
                    # Use target characteristic, or fall back to the first available one
                    selected_char = (
                        target_char if target_char else notification_chars[0]
                    )

                    print(
                        f"\nSelected monitoring characteristic: {selected_char.uuid} (Handle: 0x{selected_char.handle:04x})"
                    )
                    await client.start_notify(selected_char, self.notification_handler)

                    # If there are multiple notification characteristics, try monitoring others too
                    other_chars = [
                        char for char in notification_chars if char != selected_char
                    ]
                    for char in other_chars[:2]:  # Monitor up to 2 additional characteristics
                        try:
                            print(
                                f"Also monitoring: {char.uuid} (Handle: 0x{char.handle:04x})"
                            )
                            await client.start_notify(char, self.notification_handler)
                        except Exception as e:
                            print(f"Failed to monitor characteristic {char.uuid}: {e}")

                    print("\nMonitoring started, press Ctrl+C to exit...")
                    print("Waiting for speed data...")
                    self.running = True

                    # Keep connection alive and listen for data
                    while self.running:
                        await asyncio.sleep(1)

                    # Stop all notifications
                    for char in notification_chars:
                        try:
                            await client.stop_notify(char)
                        except Exception:
                            pass
                    print("Monitoring stopped")
                else:
                    print("No available notification characteristics found")
                    print("Device may not support notification functionality")

        except Exception as e:
            print(f"Connection error: {e}")

    def stop_monitoring(self):
        """Stop monitoring"""
        self.running = False
        print("Stopping monitoring...")


async def main():
    """Main function"""
    monitor = BS2PROMonitor()

    # Set up signal handlers
    def signal_handler(signum, frame):
        print("\nExit signal received")
        monitor.stop_monitoring()

    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)

    try:
        # Scan for devices
        device = await monitor.scan_devices()

        if device:
            # Connect and monitor device
            await monitor.connect_and_monitor(device)
        else:
            print("Target device not found, exiting")
            return 1

    except KeyboardInterrupt:
        print("\nProgram interrupted by user")
    except Exception as e:
        print(f"Program execution error: {e}")
        return 1

    return 0


if __name__ == "__main__":
    # Check if bleak library is installed
    try:
        import importlib.util

        if importlib.util.find_spec("bleak") is None:
            raise ImportError
        print("Bleak library is installed")
    except ImportError:
        print("Error: bleak library is not installed")
        print("Please run: pip install bleak")
        sys.exit(1)

    # Run main program
    exit_code = asyncio.run(main())
    sys.exit(exit_code)
