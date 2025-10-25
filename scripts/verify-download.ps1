# ClaraCore Download Verification Script
# Helps users verify the integrity of downloaded ClaraCore binaries

param(
    [Parameter(Mandatory=$true)]
    [string]$BinaryPath,
    
    [Parameter(Mandatory=$true)]
    [string]$ExpectedHash
)

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host " ClaraCore Download Verification" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Check if file exists
if (-not (Test-Path $BinaryPath)) {
    Write-Host "❌ Error: File not found: $BinaryPath" -ForegroundColor Red
    exit 1
}

Write-Host "📁 File: $BinaryPath" -ForegroundColor White
Write-Host "🔍 Calculating SHA256 hash..." -ForegroundColor Yellow
Write-Host ""

# Calculate actual hash
$actualHash = (Get-FileHash -Path $BinaryPath -Algorithm SHA256).Hash.ToLower()
$expectedHashLower = $ExpectedHash.ToLower()

Write-Host "Expected: $expectedHashLower" -ForegroundColor White
Write-Host "Actual:   $actualHash" -ForegroundColor White
Write-Host ""

# Compare hashes
if ($actualHash -eq $expectedHashLower) {
    Write-Host "✅ SUCCESS: Hashes match!" -ForegroundColor Green
    Write-Host ""
    Write-Host "Your download is authentic and hasn't been tampered with." -ForegroundColor Green
    Write-Host ""
    
    # Check for digital signature (Windows only)
    if ($BinaryPath -like "*.exe") {
        Write-Host "🔐 Checking digital signature..." -ForegroundColor Yellow
        
        try {
            $signature = Get-AuthenticodeSignature -FilePath $BinaryPath
            
            if ($signature.Status -eq "Valid") {
                Write-Host "✅ Digital signature is VALID" -ForegroundColor Green
                Write-Host "   Signer: $($signature.SignerCertificate.Subject)" -ForegroundColor White
            } elseif ($signature.Status -eq "NotSigned") {
                Write-Host "⚠️  Binary is not digitally signed" -ForegroundColor Yellow
                Write-Host "   This is common for open source projects" -ForegroundColor Yellow
                Write-Host "   Hash verification confirms authenticity" -ForegroundColor Yellow
            } else {
                Write-Host "⚠️  Signature status: $($signature.Status)" -ForegroundColor Yellow
            }
        } catch {
            Write-Host "⚠️  Could not check signature: $($_.Exception.Message)" -ForegroundColor Yellow
        }
        Write-Host ""
    }
    
    Write-Host "========================================" -ForegroundColor Green
    Write-Host " ✅ VERIFICATION SUCCESSFUL" -ForegroundColor Green
    Write-Host "========================================" -ForegroundColor Green
    Write-Host ""
    Write-Host "The file is safe to use." -ForegroundColor Green
    Write-Host ""
    
    # Offer to add Windows Defender exclusion
    Write-Host "Would you like to add this file to Windows Defender exclusions?" -ForegroundColor Cyan
    Write-Host "This will prevent false positive warnings." -ForegroundColor Cyan
    Write-Host ""
    $response = Read-Host "Add exclusion? (y/n)"
    
    if ($response -eq "y" -or $response -eq "Y") {
        try {
            Add-MpPreference -ExclusionPath $BinaryPath
            Write-Host "✅ Exclusion added successfully!" -ForegroundColor Green
        } catch {
            Write-Host "⚠️  Could not add exclusion. Try running PowerShell as Administrator." -ForegroundColor Yellow
        }
    }
    
    exit 0
    
} else {
    Write-Host "❌ FAILED: Hashes do NOT match!" -ForegroundColor Red
    Write-Host ""
    Write-Host "========================================" -ForegroundColor Red
    Write-Host " ⚠️  WARNING: VERIFICATION FAILED" -ForegroundColor Red
    Write-Host "========================================" -ForegroundColor Red
    Write-Host ""
    Write-Host "DO NOT USE THIS FILE!" -ForegroundColor Red
    Write-Host ""
    Write-Host "Possible reasons:" -ForegroundColor Yellow
    Write-Host "  • Download was corrupted" -ForegroundColor Yellow
    Write-Host "  • File has been tampered with" -ForegroundColor Yellow
    Write-Host "  • Wrong hash provided" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "Actions to take:" -ForegroundColor White
    Write-Host "  1. Delete the file" -ForegroundColor White
    Write-Host "  2. Re-download from official GitHub release" -ForegroundColor White
    Write-Host "  3. Verify the expected hash from release notes" -ForegroundColor White
    Write-Host ""
    
    exit 1
}
