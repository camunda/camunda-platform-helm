# Add "additionalProperties": false to all non-root objects with "properties"
# This enforces strict validation for nested objects while maintaining flexibility
# at the root level for user-defined structures and YAML anchors.
#
# Usage: jq -f helm-schema-make-strict.jq values.schema.json > values.schema.strict.json

# Helper function to check if we're at root level
def is_root_properties: . | has("title") and has("type") and .type == "object";

# Recursively walk the schema
walk(
  if type == "object" and has("properties") then
    # Check if this is the root schema object (has title and is top-level object)
    if is_root_properties then
      # At root level, do not add additionalProperties: false
      # This maintains flexibility for user-defined structures and YAML anchors
      .
    else
      # For all other objects with properties, add additionalProperties: false
      if has("additionalProperties") then
        # Already has additionalProperties, don't override
        .
      else
        # Add additionalProperties: false to enforce strict validation
        . + {"additionalProperties": false}
      end
    end
  else
    .
  end
)
