#!/usr/bin/env python3
"""
Script to add 'additionalProperties: false' to all object types in a JSON schema.
This makes the schema strict by rejecting any properties not explicitly defined.
"""

import json
import sys
from pathlib import Path


def add_additional_properties(obj, path="root"):
    """
    Recursively add additionalProperties: false to all objects in the schema.
    
    Args:
        obj: The object to process (dict, list, or primitive)
        path: Current path in the schema (for debugging)
    
    Returns:
        The modified object
    """
    if isinstance(obj, dict):
        # If it's an object type without additionalProperties, add it
        if obj.get("type") == "object" and "additionalProperties" not in obj:
            obj["additionalProperties"] = False
            print(f"  Added additionalProperties: false to {path}")
        
        # Recursively process all properties
        for key, value in list(obj.items()):
            new_path = f"{path}.{key}"
            obj[key] = add_additional_properties(value, new_path)
    
    elif isinstance(obj, list):
        return [add_additional_properties(item, f"{path}[{i}]") 
                for i, item in enumerate(obj)]
    
    return obj


def main():
    if len(sys.argv) < 2:
        print("Usage: python make-schema-strict.py <input-schema.json> [output-schema.json]")
        sys.exit(1)
    
    input_file = Path(sys.argv[1])
    
    # Default output file: add "-strict" suffix
    if len(sys.argv) > 2:
        output_file = Path(sys.argv[2])
    else:
        output_file = input_file.parent / f"{input_file.stem}-strict{input_file.suffix}"
    
    # Check if input file exists
    if not input_file.exists():
        print(f"❌ Error: File not found: {input_file}")
        sys.exit(1)
    
    print(f"📖 Reading schema from: {input_file}")
    
    try:
        with open(input_file, 'r', encoding='utf-8') as f:
            schema = json.load(f)
    except json.JSONDecodeError as e:
        print(f"❌ Error: Invalid JSON in {input_file}: {e}")
        sys.exit(1)
    
    print(f"🔧 Processing schema...")
    strict_schema = add_additional_properties(schema)
    
    print(f"💾 Writing strict schema to: {output_file}")
    with open(output_file, 'w', encoding='utf-8') as f:
        json.dump(strict_schema, f, indent=4, ensure_ascii=False)
    
    print(f"✅ Done! Strict schema created: {output_file}")
    print(f"\nThe schema now rejects any properties not explicitly defined.")


if __name__ == "__main__":
    main()
