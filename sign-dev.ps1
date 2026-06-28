$cert = Get-ChildItem -Path Cert:\CurrentUser\My | Where-Object { $_.Subject -eq "CN=gix-dev, O=gix, C=BR" }
if (-not $cert) {
    Write-Error "Certificado gix-dev nao encontrado. Crie um com: New-SelfSignedCertificate -Type CodeSigningCert -Subject 'CN=gix-dev,O=gix,C=BR' -CertStoreLocation Cert:\CurrentUser\My -TextExtension '2.5.29.37={text}1.3.6.1.5.5.7.3.3'"
    exit 1
}
Set-AuthenticodeSignature -FilePath "$PSScriptRoot\bin\gix.exe" -Certificate $cert -ErrorAction Stop
Write-Output "gix.exe assinado com sucesso"
