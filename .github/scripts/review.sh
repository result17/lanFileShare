#!/bin/bash

# set -e: Exit immediately if a command exits with a non-zero status.
# set -o pipefail: Ensures that a command in a pipeline failing will cause the entire pipeline to fail.
set -eo pipefail

# --- Configuration ---
# A list of file extensions to review, separated by |
FILE_EXTENSIONS_TO_REVIEW="go"

# --- Main Script ---

# 1. Find changed files.
# Using a 'while read -r' loop is the safest way to process file lists in shell scripts.
# It correctly handles filenames with spaces or other special characters.
echo "Finding changed files..."
git diff --name-only "$BASE_SHA" "$HEAD_SHA" | grep -E "\.($FILE_EXTENSIONS_TO_REVIEW)$" | while read -r FILE; do
  # In case of blank lines from the pipe, skip.
  if [ -z "$FILE" ]; then
    continue
  fi

  echo "-----------------------------------------------------"
  echo "Reviewing file: $FILE"

  # Read the content of the file. This is the correct way, it does not execute the file.
  FILE_CONTENT=$(cat "$FILE")
  if [ -z "$FILE_CONTENT" ]; then
    echo "File is empty. Skipping."
    continue
  fi

  # 3. Construct the prompt for the Gemini model.
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

  # 4. Call the Gemini CLI.
  echo "Sending to Gemini for review..."
  REVIEW_COMMENT=$(echo "$PROMPT" | gemini)

  # 5. Format the review comment body.
  COMMENT_BODY=$(cat <<EOF
### 🤖 Gemini Review for \`$FILE\`

$REVIEW_COMMENT
EOF
)

  # 6. <<< CRITICAL FIX HERE >>>
  # Create a valid JSON payload using the 'jq' utility.
  # This correctly escapes all special characters (newlines, quotes, etc.)
  # in the COMMENT_BODY, which fixes the "error: 400" from curl.
  JSON_PAYLOAD=$(jq -n --arg body "$COMMENT_BODY" '{body: $body}')

  # 7. Post the comment to the PR using the safely-built JSON payload.
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