#!/bin/bash
set -x -e 
# Define directories to ignore
ignore_dirs="./test"

# Get the current date in MM-DD-YYYY format
current_date=$(date +"%m-%d-%Y")

# Path to the log file with the current date
log_file="development_notes_cleanup_${current_date}.log"

files_to_process=($(grep -Rl -e "TODO:" -e "NOTE:" --exclude-dir="$ignore_dirs" . | grep -v remove_notes_todo))

# Define the awk command as a variable for readability
awk_command='
    /# TODO:/ || /# NOTE:/ {
        noteTodoIndent = match($0, /(# TODO:|# NOTE:)/);
        processing = 1;
        next;
    }
    /{{\/\* TODO:/ || /{{\/\* NOTE:/ {
        deleteUntilEnd = 1;
    }
    deleteUntilEnd {
        if (/.*\*\/}}/) {
            deleteUntilEnd = 0;
        }
        next;
    }
    processing && /^ *#/ {
        hashIndent = match($0, "[^ ]");  # Find first non-space character
        if (hashIndent == noteTodoIndent) {
            # If indentation matches, skip this line and stop processing
            processing = 0;
            next;
        }
    }
    {
        # Reset processing and print non-matching lines
        processing = 0;
        print;
    }
'

# Empty or create the log file
> "$log_file"

# Loop through the files and process them with awk
for file in "${files_to_process[@]}"; do
    echo "Processing $file..."

    # Create a unique temporary file name
    temp_file="${file}.temp"

    # Process the file with awk
    echo "$awk_command" | awk -f - "$file" > "$temp_file"

    # Log the changes
    echo "Changes in $file:" >> "$log_file"
    diff "$file" "$temp_file" >> "$log_file" || echo "No changes in $file" >> "$log_file"

    # Replace the original file
    mv "$temp_file" "$file"
done

echo "Processing complete."



