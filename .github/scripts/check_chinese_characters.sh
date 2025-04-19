#!/bin/bash

found_chinese=false
while IFS= read -r line; do
    if [ -n "$line" ]; then
        echo "Chinese chars found in $line"
        found_chinese=true
    fi
done < <(find . \( -name "*.go" -o -name "*.cpp" -o -name "*.h" -o -name "*.hpp" \
    -o -name "*.sol" -o -name "*.js" -o -name "*.jsx" -o -name "*.ts" \
    -o -name "*.tsx" -o -name "Makefile" -o -name "*.mk" \
    -o -name "Dockerfile" -o -name "*.md" \
    -o -name "*.yaml" -o -name "*.yml" \) \
    -type f -exec sh -c 'grep -H -n -P "[\x{4e00}-\x{9fa5}]" "$0"' {} \;)
if [ "$found_chinese" = true ]; then
    echo "Error: Found Chinese characters in source files!"
    exit 1
fi