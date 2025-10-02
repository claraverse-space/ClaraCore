# GitHub Token Setup for ClaraCore Releases

To use the release script, you need a GitHub Personal Access Token with repository access.

## Creating a GitHub Token

1. Go to [GitHub Settings > Personal Access Tokens > Tokens (classic)](https://github.com/settings/tokens)

2. Click "Generate new token (classic)"

3. Configure the token:
   - **Note**: ClaraCore Release Management
   - **Expiration**: Choose an appropriate expiration (90 days recommended)
   - **Scopes**: Select the following:
     - ✅ `repo` (Full control of private repositories)
       - This includes: repo:status, repo_deployment, public_repo, repo:invite, security_events

4. Click "Generate token"

5. **Important**: Copy the token immediately! You won't be able to see it again.

## Using the Token

### Option 1: Token File (Recommended)
```bash
# Save token to file
echo "ghp_your_token_here" > .github_token

# Use with release script
python release.py --version v0.1.0 --token-file .github_token
```

### Option 2: Command Line
```bash
# Pass token directly
python release.py --version v0.1.0 --token ghp_your_token_here
```

### Option 3: Environment Variable
```bash
# Set environment variable
export GITHUB_TOKEN=ghp_your_token_here

# Modify release.py to read from environment if needed
```

## Security Notes

- Keep your token secure and never commit it to version control
- Add `.github_token` to your `.gitignore` file
- Use minimal required permissions
- Regenerate tokens periodically
- Consider using fine-grained tokens for better security

## Token Permissions Required

The release script needs these permissions:
- **Contents**: Read and write (for creating releases)
- **Metadata**: Read (for repository information)
- **Pull requests**: Read (for changelog generation)

## Troubleshooting

### "Bad credentials" error
- Check that your token is correct
- Ensure the token hasn't expired
- Verify the token has the required `repo` scope

### "Not found" error
- Check that the repository name is correct in the script
- Ensure your token has access to the repository
- Verify you're the owner or have collaborator access

### Rate limiting
- GitHub has rate limits for API calls
- If you hit limits, wait and try again
- Consider using a token with higher limits

## Example Usage

```bash
# Create release with token file
python release.py --version v0.1.0 --token-file .github_token

# Create draft release
python release.py --version v0.1.0 --token-file .github_token --draft

# Build only (no release)
python release.py --version v0.1.0 --token-file .github_token --build-only
```