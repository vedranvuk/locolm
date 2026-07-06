# Todo: Enhance File Manipulation Tools for Agentic Coding

To improve the agent's ability to perform complex source code modifications, we will implement three new tools within the `internal/tool/fs` package. These will complement the existing `read_file`, `create_file`, and `replace_string_in_file` operations.

## Tasks

### 1. Implement `edit_range`
- **Goal**: Replace a specific range of lines with new content without needing to identify an exact "old string."
- **Location**: `internal/tool/fs/fs.go`
- **Implementation Details**:
  - Follow the existing `fsRead` logic to get a slice of lines: `lines := strings.Split(content, "\n")`.
  - Implement a handler that takes `startLine`, `endLine`, and `newContent`.
  - The range is 1-indexed (matching `fs_read`).
  - **Code Snippet**:
    ```go
    func fsEditRange(args map[string]string) (string, error) {
        path := args["path"]
        startStr := args["startLine"]
        endStr := args["endLine"]
        newContent := args["newContent"]

        resolved, err := resolveAndValidate(path)
        if err != nil { return "", err }

        // Read full content to get all lines
        content, err := os.ReadFile(resolved)
        if err != nil { return "", err }
        lines := strings.Split(string(content), "\n")

        start, _ := strconv.Atoi(startStr)
        end, _ := strconv.Atoi(endStr)

        // Replace slice [start-1 : end] with lines of newContent
        newLines := strings.Split(newContent, "\n")
        for i, line := range newLines {
            if (i + start - 1) < len(lines) {
                lines[i+start-1] = line
            }
        }

        // Handle cases where new content is longer/shorter than old range
        // If we want to strictly replace the segment:
        result := strings.Join(lines[start-1 : end], "\n") // placeholder logic
        // For a true replacement of a range with multiple lines:
        finalLines := append(lines[:start-1], newLines...)
        finalLines = append(finalLines, lines[end:]...)

        return strings.Join(finalLines, "\n"), nil
    }
    ```
- **Test Command**: `go run cmd/locolm/main.go` (Verify by checking if the range is correctly replaced).

### 2. Implement `append_line` & `prepend_line`
- **Goal**: Efficiently add content to the boundaries of a file.
- **Location**: `internal/tool/fs/fs.go`
- **Implementation Details**:
  - Use `os.ReadFile` and `os.WriteFile` for simplicity, or append to current slice of lines.
  - **AppendLine**: Join existing content with `\n`, then add `newContent`.
  - **PrependLine**: Add `content + \n` at the beginning of the file.
- **Test Command**: `go run cmd/locolm/main.go` (Verify by checking first/last line).

### 3. Implement `move_file` & `rename_file`
- **Goal**: Provide explicit tools for moving or renaming files to reduce reliance on raw terminal commands.
- **Location**: `internal/tool/fs/fs.go`
- **Implementation Details**:
  - Use `os.Rename(oldPath, newPath)`.
  - Ensure the parent directory of `newPath` is created using `os.MkdirAll` if it doesn't exist (similar to how `fs_write` handles the current path).
- **Test Command**: `go run cmd/locolm/main.go` (Verify file exists at new path and not at old).

## Documentation Update
- Once implemented, update `docs/tools.md` to include definitions for these new tools so the agent can utilize them effectively.