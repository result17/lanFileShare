#!/bin/bash

# set -x: Print commands for debugging (optional, remove for production).
# set -e: Exit immediately if a command exits with a non-zero status.
# set -o pipefail: The return value of a pipeline is the status of the last command to exit with a non-zero status.
set -eo pipefail

# --- Configuration ---
FILE_EXTENSIONS_TO_REVIEW="go"

# --- Main Script ---

echo "Finding changed files..."

# --- FIX START ---
# Run the command to get changed files, but add '|| true' to the end of the
# grep command. This ensures that even if grep finds nothing (and exits with 1),
# the pipeline will continue and not trigger 'set -e'.
CHANGED_FILES=$(git diff --name-only "$BASE_SHA" "$HEAD_SHA" | grep -E "\.($FILE_EXTENSIONS_TO_REVIEW)$" || true)

# Now, check if the CHANGED_FILES variable is empty.
if [ -z "$CHANGED_FILES" ]; then
  echo "No files with specified extensions (.${FILE_EXTENSIONS_TO_REVIEW}) changed. Skipping review."
  exit 0 # Exit successfully.
fi
# --- FIX END ---

echo "Found changed files to review:"
echo "$CHANGED_FILES"

# Use a while-read loop to process the files from the variable.
echo "$CHANGED_FILES" | while read -r FILE; do
  if [ -z "$FILE" ]; then
    continue
  fi

  echo "-----------------------------------------------------"
  echo "Reviewing file: $FILE"

  FILE_CONTENT=$(cat "$FILE")
  if [ -z "$FILE_CONTENT" ]; then
    echo "File is empty. Skipping."
    continue
  fi

  # ... (The rest of your script remains the same) ...

  # Construct the prompt for the Gemini model.
  PROMPT=$(cat <<EOF
You are an expert Go programmer acting as a senior code reviewer for a project named "lanFileSharer".
Your task is to provide a concise and constructive code review for the following file: \`$FILE\`.
Focus on the following areas:
- **Potential Bugs:** Identify any logic errors, race conditions, or unhandled edge cases.
- **Best Practices:** Check for adherence to idiomatic Go practices (e.g., error handling, interface usage, package design).
- **Clarity & Readability:** Suggest improvements to make the code easier to understand and maintain.
- **Performance:** Point out any obvious performance bottlenecks, but avoid premature optimization.
- **Security:** Highlight any potential security vulnerabilities.
**Review Guidelines:**
- Provide actionable feedback. Instead of just saying "this is wrong," explain why and suggest a better approach.
- Use short code snippets to illustrate your points where applicable.
- Be concise. Group related comments together.
- If the code is excellent and requires no changes, your ONLY response should be: "LGTM! (Looks Good To Me) 👍"
- Do not comment on minor style issues like whitespace or missing comments, as linters already handle those.
Here is the code for \`$FILE\`:
\`\`\`go
$FILE_CONTENT
\`\`\`
EOF
)

  # Call the Gemini CLI.
  echo "Sending to Gemini for review..."
  REVIEW_COMMENT=$(echo "$PROMPT" | gemini)

  # Format the review comment body.
  COMMENT_BODY=$(cat <<EOF
### 🤖 Gemini Review for \`$FILE\`

$REVIEW_COMMENT
EOF
)

  # Create a valid JSON payload using jq.
  JSON_PAYLOAD=$(jq -n --arg body "$COMMENT_BODY" '{body: $body}')

  # Post the comment to the PR.
  echo "Posting review comment to PR #$PR_NUMBER..."
  curl -s -S -f -X POST \
    -H "Authorization: Bearer $GITHUB_TOKEN" \
    -H "Accept: application/vnd.github.v3+json" \
    "https://api.github.com/repos/${GITHUB_REPOSITORY}/issues/$PR_NUMBER/comments" \
    -d "$JSON_PAYLOAD"

  echo "Review for $FILE posted successfully."
done

echo "-----------------------------------------------------"
echo "All changed files have been reviewed."