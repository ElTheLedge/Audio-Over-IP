# Build Directory

The build directory is used to house all the build files and assets for the application (Wails v3).

This application is Windows-only (it relies on the Windows Core Audio API), so only the
Windows build files are present.

The structure is:

* bin - Output directory. `wails3 build` produces the standalone `AudioOverIP.exe` here.
* config.yml - Project configuration (product info, dev mode). After changing `info`, run
  `wails3 task common:update:build-assets` to regenerate the assets below.
* Taskfile.yml - Shared build tasks (frontend build, bindings, icons, ...)
* windows - Windows specific files and tasks

## Windows

The app ships as a single, self-contained executable built with `wails3 build` (via
`wails3 task build`). No installer is used - NSIS packaging is not part of this project.

The `windows` directory contains the manifest and resource files used when building. These may be
customised for your application. To return these files to the default state, run
`wails3 task common:update:build-assets`; it deletes the platform assets this Windows-only app
never uses afterwards.

- `icon.ico` - The icon used for the application. It is generated from the `appicon.png` file in this
  directory (`wails3 task common:generate:icons`). If you wish to use a different icon, replace
  `appicon.png`.
- `info.json` - Application details used for Windows builds. The data here is shown by the executable
  itself (right click the exe -> properties -> details).
- `wails.exe.manifest` - The main application manifest file.

MSIX packaging remains available as an optional extra (`wails3 task package`, which runs
`wails3 tool msix`). The CLI renders the MSIX manifest from the metadata in `build/config.yml`,
so this project keeps no MSIX files of its own.
