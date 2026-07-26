# Antivirus False Positives - JarvisCore

## Why Am I Getting Antivirus Warnings?

If you're seeing warnings from Windows Defender or other antivirus software when downloading or running JarvisCore, **don't worry** - this is a common issue with legitimate software, especially:

- 🆕 **New releases** without established reputation scores
- 🔧 **System tools** that manage processes and network operations
- 🐹 **Go binaries** which sometimes trigger heuristic detection
- 📦 **Unsigned executables** (code signing certificates are expensive for open source projects)

## Is JarvisCore Safe?

**Yes!** JarvisCore is completely safe and open source:

✅ **100% Open Source** - Every line of code is public on GitHub  
✅ **No Malicious Code** - You can audit the entire codebase yourself  
✅ **Reproducible Builds** - Build from source to verify authenticity  
✅ **Active Community** - Transparent development and issue tracking  
✅ **MIT Licensed** - Permissive open source license  

## What JarvisCore Actually Does

JarvisCore is an AI inference server that:
- Starts local HTTP servers for AI model hosting
- Downloads AI models from HuggingFace
- Manages GPU/CPU resources
- Provides OpenAI-compatible API endpoints

These legitimate operations can trigger antivirus heuristics designed to catch malware.

## How to Verify Your Download

### 1. Check SHA256 Hash

Every release includes SHA256 checksums. Verify your download:

**Windows PowerShell:**
```powershell
Get-FileHash jarviscore-windows-amd64.exe -Algorithm SHA256
```

**Linux/macOS:**
```bash
sha256sum jarviscore-linux-amd64
```

Compare the output with the checksum in the GitHub release notes.

### 2. Verify Digital Signature (if available)

If the binary is code signed:

**Windows:**
1. Right-click `jarviscore-windows-amd64.exe`
2. Select **Properties**
3. Go to **Digital Signatures** tab
4. Verify the signature is from "JarvisCore Open Source"

### 3. Build from Source

The ultimate verification - build it yourself:

```bash
git clone https://github.com/jarvisverse-space/JarvisCore.git
cd JarvisCore
go build .
```

You'll get an identical binary (minus metadata).

## Resolving False Positives

### Option 1: Add Exclusion to Windows Defender

1. Open **Windows Security**
2. Go to **Virus & threat protection**
3. Click **Manage settings**
4. Scroll to **Exclusions** → **Add or remove exclusions**
5. Add the JarvisCore executable or installation folder

### Option 2: Temporarily Disable Real-time Protection

While downloading:
1. Open **Windows Security**
2. Go to **Virus & threat protection**
3. Click **Manage settings**
4. Toggle off **Real-time protection** temporarily
5. Download and verify JarvisCore
6. Re-enable protection

### Option 3: Report False Positive to Microsoft

Help improve detection for everyone:

1. Visit [Microsoft Security Intelligence](https://www.microsoft.com/en-us/wdsi/filesubmission)
2. Submit the JarvisCore executable
3. Select **"I believe this file does not contain a threat"**
4. Microsoft will analyze and update their database

## For Other Antivirus Software

### VirusTotal Scanning

If you want multiple opinions, scan with VirusTotal:
- Visit https://www.virustotal.com
- Upload the executable
- Review results from 70+ antivirus engines

**Note:** Some engines will still flag it as potentially unwanted due to:
- Generic heuristics for network/process operations
- Lack of code signing
- New file without reputation

### Popular Antivirus Products

**Norton/Symantec:**
- Add to exclusions via Settings → Antivirus → Scans and Risks → Exclusions

**McAfee:**
- Add to exclusions via Settings → Real-Time Scanning → Excluded Files

**Avast/AVG:**
- Add to exclusions via Settings → General → Exceptions

**Bitdefender:**
- Add to exclusions via Protection → Antivirus → Exclusions

## Why Don't We Just Sign the Binary?

**Code signing certificates are expensive** - typically $200-500 per year from certificate authorities. As an open source project, this is a significant cost.

We're exploring options:
- Applying for free code signing through open source programs
- Community sponsorship for certificate costs
- Building reputation over time to reduce false positives

**If you can help sponsor a code signing certificate, please contact us!**

## Technical Details: What Triggers Detection?

JarvisCore performs legitimate operations that antivirus software monitors:

1. **Network Operations**
   - Creates HTTP/HTTPS servers
   - Downloads files from HuggingFace
   - Makes API requests

2. **Process Management**
   - Spawns llama-server processes
   - Monitors and manages subprocesses
   - Handles process cleanup

3. **File System Access**
   - Reads/writes configuration files
   - Downloads and stores AI models
   - Creates log files

4. **System Information**
   - Checks GPU availability
   - Monitors memory usage
   - Detects CUDA/ROCm/Metal

These are normal operations for system tools and servers, but can trigger heuristic detection in antivirus software.

## Our Security Practices

We take security seriously:

- ✅ **Public Development** - All commits are visible on GitHub
- ✅ **Code Review** - Changes are reviewed before merging
- ✅ **Dependency Management** - Go modules with version pinning
- ✅ **No Telemetry** - JarvisCore doesn't phone home
- ✅ **Local First** - Everything runs on your machine
- ✅ **Transparent Builds** - Reproducible build process

## Still Concerned?

We understand security is important. If you're still unsure:

1. **Read the Source Code** - It's all on GitHub
2. **Check the Issues** - See how we handle security reports
3. **Build from Source** - Compile it yourself
4. **Run in a VM** - Test in an isolated environment first
5. **Ask the Community** - Join our discussions on GitHub

## Reporting Real Security Issues

If you discover an **actual security vulnerability** (not a false positive), please:

1. **DO NOT** open a public issue
2. Email security concerns to the maintainers
3. Follow responsible disclosure practices
4. We'll work with you to resolve legitimate issues

## Further Reading

- [Microsoft Defender SmartScreen](https://docs.microsoft.com/en-us/windows/security/threat-protection/microsoft-defender-smartscreen/microsoft-defender-smartscreen-overview)
- [Understanding False Positives](https://support.microsoft.com/en-us/windows/protect-your-pc-from-potentially-unwanted-applications-c2b5a5a8-5b9b-4b0c-8c6a-89f1b0f6e0f1)
- [Code Signing for Open Source](https://www.ssl.com/article/code-signing-for-open-source-projects/)

## Help Us Improve

If you encounter false positives:
- ⭐ Star the repo to improve reputation
- 📤 Submit binaries to Microsoft/antivirus vendors
- 💬 Share your experience in GitHub Discussions
- 💰 Sponsor code signing certificate costs

---

**Remember:** False positives are frustrating but common. We're a legitimate open source project committed to transparency and security. Thank you for using JarvisCore!
