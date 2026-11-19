# Build and assemble the portable folder

## Build the Windows interface

Prerequisites are Go 1.27.4 and Git. From the repository root in PowerShell:

```powershell
Push-Location src
go run github.com/akavel/rsrc@v0.10.2 -arch amd64 -manifest app.manifest -ico assets/app.ico -o app_windows_amd64.syso
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
go build -buildvcs=false -trimpath -ldflags '-s -w -H windowsgui' -o '../portable/PS1 to EBOOT.exe' .
Pop-Location
New-Item -ItemType Directory -Force 'portable/Artwork Defaults' | Out-Null
Copy-Item artwork/*.png 'portable/Artwork Defaults/'
```

The prebuilt portable ZIP already includes the executable, artwork defaults, patched engine, and .NET runtime.

## Add the conversion engine

Build the patched engine with the .NET 10 SDK. The upstream commit is pinned to the version used by this package.

```powershell
git clone https://github.com/RupertAvery/PSXPackager.git upstream
git -C upstream checkout 63cfacfe29b4ec6668a05f719e5451b5d861d1b0
git -C upstream apply ../patches/redirected-console.patch ../patches/gui-boot-resources.patch
dotnet publish upstream/PSXPackager/PSXPackager-windows.csproj -c Release -r win-x64 -o engine-build
New-Item -ItemType Directory -Force portable/Engine/Resources | Out-Null
Copy-Item engine-build/psxpackager.exe portable/Engine/
Copy-Item artwork/*.png portable/Engine/Resources/
```

Download the Windows x64 **ZIP** for the [.NET 10 runtime](https://dotnet.microsoft.com/en-us/download/dotnet/10.0) and extract its contents into `portable/Runtime/`.

On first launch, the app imports the three required boot resources from the official PSXPackager v1.8.0 ZIP.

## Optional PMF tools

- Save the [Gyan FFmpeg 9.0.2 Essentials 7z](https://www.gyan.dev/ffmpeg/builds/packages/ffmpeg-9.0.2-essentials_build.7z) as `portable/Tools/FFmpeg/ffmpeg-9.0.2-essentials_build.7z`. Its SHA-256 is `4705843ccaaf54257c16ad90f3e952ece33c17df964ecf7bfdbb0f49c7171077`.
- Extract the [Windows x64 embeddable Python 3.13.9 ZIP](https://www.python.org/ftp/python/3.13.9/python-3.13.9-embed-amd64.zip) into `portable/Tools/Python/`.
- Save [pspmedia.py from Total Kommando's PSP Media Toolkit](https://raw.githubusercontent.com/TotalKommando/psp-media-toolkit/3e4825e60823fb9364284030e8baffea542ea546/pspmedia.py) into `portable/Tools/`.

The assembled folder is ready to launch on Windows. Keep its contents together. The media tools are optional for disc conversion.
