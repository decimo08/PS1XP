# PS1XP v0.1.0-rc.3

- The Windows x64 portable ZIP includes the three PSXPackager conversion resources alongside the patched engine and .NET runtime.
- Extract the ZIP and run PS1 to EBOOT.exe. The former first-run download prompt has been removed.
- The app checks the bundled resource hashes on startup. Disc conversion runs offline.
- Online cover lookup still uses the internet. Optional PMF tooling is documented in BUILD.md.

The resource files match the official PSXPackager v1.8.0 distribution. The package is unsigned and still needs a Windows UI and physical PSP smoke test.
