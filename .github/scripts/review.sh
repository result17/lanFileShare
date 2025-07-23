#!/bin/bash

# Exit immediately if a command exits with a non-zero status.
set -e

# --- Configuration ---
# A list of file extensions to review.
# Add or remove extensions as needed, e.g., "go|ts|py|java"
FILE_EXTENSIONS_TO_REVIEW="go"

# --- Main Script ---

# 1. Find all files that have changed in this PR for the specified extensions.
# We use git diff between the base and head of the pull request.
echo "Finding changed files..."
CHANGED_FILES=$(git diff --name-only "$BASE_SHA" "$HEAD_SHA" | grep -E "\.($FILE_EXTENSIONS_TO_REVIEW)$" || true)

if [ -z "$CHANGED_FILES" ]; then
  echo "No files with specified extensions changed. Skipping review."
  exit 0
fi

echo -e "Found changed files to review:\n$CHANGED_FILES"

# 2. Loop through each changed file and send it for review.
for FILE in $CHANGED_FILES; do
  echo "-----------------------------------------------------"
  echo "Reviewing file: $FILE"

  # Read the content of the file.
  FILE_CONTENT=$(cat "$FILE")
  if [ -z "$FILE_CONTENT" ]; then
    echo "File is empty. Skipping."
    continue
  fi

  # 3. Construct the prompt for the Gemini model.
  # This is a critical part. A good prompt leads to good reviews.
  PROMPT=$(cat <<EOF
You are an expert Go programmer acting as a senior code reviewer for a project named "lanFileSharer".
Your task is to provide a concise and constructive code review for the following file: `$FILE`.

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

Here is the code for `$FILE`:
```go
$FILE_CONTENT
```
EOF
)

  # 4. Call the Gemini CLI.
  # The prompt is piped to the standard input of the CLI.
  # The GEMINI_API_KEY is used by the CLI for authentication.
  echo "Sending to Gemini for review..."
  REVIEW_COMMENT=$(echo "$PROMPT" | gemini)

  # 5. Format the review comment and post it to the GitHub Pull Request.
  # The GITHUB_TOKEN is used for authenticating with the GitHub API.
  COMMENT_BODY=$(cat <<EOF
### 🤖 Gemini Review for `$FILE`

$REVIEW_COMMENT
EOF
)

  echo "Posting review comment to PR #$PR_NUMBER..."
  curl -s -S -f -X POST \
    -H "Authorization: Bearer $GITHUB_TOKEN" \
    -H "Accept: application/vnd.github.v3+json" \
    "https://api.github.com/repos/${GITHUB_REPOSITORY}/issues/$PR_NUMBER/comments" \
    -d @- <<EOF
{
  "body": "$COMMENT_BODY"
}
EOF

  echo "Review for $FILE posted successfully."
done

echo "-----------------------------------------------------"
echo "All changed files have been reviewed."
