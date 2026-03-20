import asyncio
from winrt.windows.devices.bluetooth import BluetoothLEDevice, BluetoothCacheMode
from winrt.windows.devices.bluetooth.genericattributeprofile import (
    GattCharacteristicProperties,
)
from winrt.windows.storage.streams import DataWriter, Buffer
from winrt.windows.devices.enumeration import DeviceInformation

# Add HID support
try:
    import hid

    HID_AVAILABLE = True
    from hid_controller import BS2PROHIDController
except ImportError:
    HID_AVAILABLE = False
    print("Warning: hidapi not installed, HID mode unavailable. Run 'pip install hidapi' to install.")


async def find_paired_ble_devices():
    """Find all paired BLE devices"""
    try:
        # Use device enumerator to find all BLE devices
        device_selector = BluetoothLEDevice.get_device_selector()
        devices = await DeviceInformation.find_all_async()

        print(f"Found {len(devices)} BLE devices:")

        for device in devices:
            if device.name:  # Only show devices with names
                print(f"Device Name: {device.name}")
                print(f"Device ID: {device.id}")
                print(f"Enabled: {device.is_enabled}")
                print(f"Paired: {device.pairing.is_paired}")
                print("---")

                # If BS2PRO device is found
                if "BS2PRO" in device.name.upper():
                    print(f"Found target device: {device.name}")
                    return device.id

        return None

    except Exception as e:
        print(f"Device enumeration error: {e}")
        return None


def analyze_characteristic_properties(properties):
    """Analyze characteristic properties"""
    prop_list = []

    # Use GattCharacteristicProperties enum values
    if properties & GattCharacteristicProperties.READ:
        prop_list.append("READ")
    if properties & GattCharacteristicProperties.WRITE:
        prop_list.append("WRITE")
    if properties & GattCharacteristicProperties.WRITE_WITHOUT_RESPONSE:
        prop_list.append("WRITE_NO_RESPONSE")
    if properties & GattCharacteristicProperties.NOTIFY:
        prop_list.append("NOTIFY")
    if properties & GattCharacteristicProperties.INDICATE:
        prop_list.append("INDICATE")
    if properties & GattCharacteristicProperties.BROADCAST:
        prop_list.append("BROADCAST")
    if properties & GattCharacteristicProperties.EXTENDED_PROPERTIES:
        prop_list.append("EXTENDED")
    if properties & GattCharacteristicProperties.AUTHENTICATED_SIGNED_WRITES:
        prop_list.append("AUTH_SIGNED_WRITES")

    return prop_list


def get_service_description(uuid_str):
    """Get standard service description"""
    standard_services = {}
    return standard_services.get(uuid_str.lower(), "Unknown Service")


def format_gatt_status(code: int) -> str:
    """Format GATT status code"""
    mapping = {
        0: "Success",
        1: "Unreachable",
        2: "ProtocolError",
        3: "AccessDenied",
    }
    return mapping.get(int(code), f"Unknown({code})")


async def discover_services_and_characteristics(device):
    """Discover all services and characteristics of the device"""
    try:
        print(f"\nAnalyzing device: {device.name}")
        print(f"Device address: {hex(device.bluetooth_address)}")
        print("=" * 60)

        # Get GATT services
        gatt_result = await device.get_gatt_services_async()

        if gatt_result.status != 0:
            print(f"Failed to get services, status code: {gatt_result.status}")
            return

        services = gatt_result.services
        print(f"Found {len(services)} services:\n")

        for i, service in enumerate(services, 1):
            service_uuid = str(service.uuid)
            service_desc = get_service_description(service_uuid)

            print(f"Service {i}: {service_desc}")
            print(f"   UUID: {service_uuid}")

            # Get characteristics (try uncached mode)
            try:
                char_result = await service.get_characteristics_async(
                    BluetoothCacheMode.UNCACHED
                )
            except TypeError:
                char_result = await service.get_characteristics_async()

            if char_result.status == 0:
                characteristics = char_result.characteristics
                print(f"   Characteristic count: {len(characteristics)}")

                for j, char in enumerate(characteristics, 1):
                    char_uuid = str(char.uuid)
                    properties = analyze_characteristic_properties(
                        char.characteristic_properties
                    )

                    print(f"      Characteristic {j}:")
                    print(f"         UUID: {char_uuid}")
                    print(f"         Properties: {', '.join(properties)}")

                    # Indicate read/write capabilities
                    capabilities = []
                    if "READ" in properties:
                        capabilities.append("Readable")
                    if "WRITE" in properties or "WRITE_NO_RESPONSE" in properties:
                        capabilities.append("Writable")
                    if "NOTIFY" in properties:
                        capabilities.append("Notifiable")
                    if "INDICATE" in properties:
                        capabilities.append("Indicatable")

                    if capabilities:
                        print(f"         Capabilities: {' | '.join(capabilities)}")

                    # If readable, try to read descriptors
                    try:
                        descriptors_result = await char.get_descriptors_async()
                        if (
                            descriptors_result.status == 0
                            and len(descriptors_result.descriptors) > 0
                        ):
                            print(
                                f"         Descriptors: {len(descriptors_result.descriptors)}"
                            )
                    except:  # noqa: E722
                        pass

                    print()
            else:
                status_desc = format_gatt_status(char_result.status)
                print(f"   Unable to get characteristics, status: {status_desc}")

                # If this is an HID service and access is denied, suggest using HID mode
                if (
                    service_uuid == "00001812-0000-1000-8000-00805f9b34fb"
                    and char_result.status == 3
                    and HID_AVAILABLE
                ):
                    print(f"   This is an HID service, it is recommended to use HID mode for access")

            print("-" * 50)

    except Exception as e:
        print(f"Service discovery error: {e}")


async def connect_to_paired_device():
    """Connect to a paired BS2PRO device"""
    try:
        # Method 1: Connect via MAC address (corrected MAC address)
        mac_address = 0xE566E510CE04  # Removed trailing 5

        try:
            print("Attempting to connect via MAC address...")
            ble_device = await BluetoothLEDevice.from_bluetooth_address_async(
                mac_address
            )

            if ble_device is not None:
                print(f"Connected via MAC address successfully: {ble_device.name}")
                await discover_services_and_characteristics(ble_device)
                return ble_device

        except Exception as mac_error:
            print(f"MAC address connection failed: {mac_error}")

        # Method 2: Find via device enumeration
        print("Attempting to find via device enumeration...")
        device_id = await find_paired_ble_devices()

        if device_id:
            ble_device = await BluetoothLEDevice.from_id_async(device_id)

            if ble_device is None:
                print("Unable to create device object")
                return None

            print(f"Connected to device: {ble_device.name}")
            await discover_services_and_characteristics(ble_device)
            return ble_device
        else:
            print("BS2PRO device not found")
            return None

    except Exception as e:
        print(f"Connection error: {e}")
        return None


# Usage example
async def main():
    """Main function"""
    print("Starting search and analysis of BS2PRO device...")
    print("\nSelect connection mode:")
    print("1. BLE GATT Mode (default)")
    if HID_AVAILABLE:
        print("2. HID Mode")

    choice = input("\nPlease select mode (1/2): ").strip()

    if choice == "2" and HID_AVAILABLE:
        # Use HID mode
        print("\nConnecting using HID mode...")
        controller = BS2PROHIDController()

        if controller.connect():
            print("\nRunning HID communication test...")
            # Specific HID command tests can be added here
            controller.disconnect()
        return

    # Default to BLE GATT mode
    print("\nConnecting using BLE GATT mode...")
    device = await connect_to_paired_device()

    if device:
        print("\nDevice analysis complete!")
        print(f"Device Name: {device.name}")
        print(f"Device Address: {hex(device.bluetooth_address)}")
        print("\nTip: Check the output above to find writable characteristic UUIDs for sending data")

        if HID_AVAILABLE:
            print("Tip: To access HID services, re-run and select HID mode")
    else:
        print("\nFailed to connect to device")
        print("\nTroubleshooting suggestions:")
        print("1. Make sure the device is paired in Windows settings")
        print("2. Make sure the device is powered on")
        print("3. Try disconnecting and reconnecting the device in Bluetooth settings")
        if HID_AVAILABLE:
            print("4. Try connecting using HID mode")


if __name__ == "__main__":
    asyncio.run(main())
