# Android direction

The Android goal is a dedicated Agent-Harness app. It is planned work;
this repository does not currently ship an Android app or APK, and there
is no committed delivery date.

The app should give people a mobile interface for composing requests,
following tool activity, reviewing approvals, interrupting work, and
resuming sessions. Installing and maintaining a Go toolchain on the phone
should not be part of that experience.

Native Termux builds follow the [release support policy](install.md#termux-android).
Termux package availability will not set the Go baseline for desktop releases.

## Decisions before implementation

- **Execution:** decide whether tools run on the device, on a connected
  development host, or through explicitly separate modes. A mobile client
  connected to a host must make the execution location visible.
- **Workspace access:** define which files the app can access, how users
  grant access, and where changes are saved.
- **Credentials and trust:** define credential storage, repository trust,
  and approval boundaries for the chosen execution model.
- **Lifecycle:** define cancellation, disconnect behavior, background work,
  and recovery after Android stops the app. Do not promise uninterrupted
  background execution.
- **Distribution:** choose supported Android versions, signing, update
  delivery, and device validation before publishing an APK.

Reuse of the Go runtime needs an explicit integration boundary. The current
terminal application is not an Android UI, and a phone-sized terminal test
does not validate an Android app.

## Evidence required to ship

Validate on Android devices: text entry with the soft keyboard, rotation,
approval and rejection, cancellation, network loss, process termination,
and session recovery. Record the tested Android versions and execution
mode. Desktop terminal tests remain useful for the CLI, but do not replace
these checks.
