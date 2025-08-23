# 📥 GitHub Actions Log Analysis Guide

## 🔍 **Method 1: GitHub Web Interface**

### Download Logs
1. Open GitHub repository page
2. Click **Actions** tab
3. Select the failed workflow run
4. Click **⚙️** icon in top right
5. Select **Download log archive**

### Quick Analysis
```bash
# Extract downloaded logs
unzip logs.zip

# Search for key failure patterns
grep -r "❌ BASIC TEST FAILED:" .
grep -r "FAILED PACKAGES:" .
grep -r "panic\|fatal" .
```

## 🔍 **Method 2: GitHub CLI (Recommended)**

### Install GitHub CLI
```bash
# macOS
brew install gh

# Windows
winget install GitHub.cli

# Ubuntu/Debian
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null
sudo apt update && sudo apt install gh
```

### Download and Analyze
```bash
# Login to GitHub
gh auth login

# Download latest failed run logs
gh run download $(gh run list --status failure --limit 1 --json databaseId --jq '.[0].databaseId')

# Quick analysis
find . -name "*.txt" -exec grep -l "❌\|FAILED" {} \;
```

## 🎯 **Quick Analysis in GitHub Web Interface**

### Key Search Terms
Use `Ctrl+F` in the GitHub Actions log page to search for:
- `❌ BASIC TEST FAILED:`
- `FAILED PACKAGES:`
- `💥 FINAL TEST RESULTS:`
- `panic` or `fatal`
- `timeout` or `deadline exceeded`

### Common Failure Patterns
```bash
# Timeout issues
Search: "timeout", "deadline exceeded", "context canceled"

# Resource contention
Search: "race", "concurrent", "goroutine"

# Environment differences
Search: "permission denied", "file not found", "network"

# Memory issues
Search: "out of memory", "killed", "signal"
```

## 🛠️ **Debug Workflow**

1. **Identify Failed Packages** → Look for `FAILED PACKAGES:` section
2. **Check Specific Errors** → Search for `❌` symbols
3. **Compare with Local** → Run `./scripts/test-like-ci.sh`
4. **Fix Issues** → Address root causes
5. **Verify Fix** → Re-run CI

## 💡 **Pro Tips**

- Use browser bookmarks for common search patterns
- Open multiple tabs to compare different log sections
- Focus on the first failure - subsequent ones might be cascading effects
- Check both individual package logs and the final summary
