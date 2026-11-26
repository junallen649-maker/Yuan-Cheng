//! Entry point for running Polaris inside `svchost.exe` (`ServiceDll` hosting).
//!
//! When a service is registered with `Parameters\ServiceDll`, `svchost.exe` loads the DLL
//! and calls its exported `ServiceMain`. The SCM has already dispatched the service at
//! that point, so this must NOT call `StartServiceCtrlDispatcherW` again; it goes straight
//! into the same body a standalone `--service` process runs.

use std::ffi::OsString;

use hbb_common::log;

/// Called by `svchost.exe` after it loads this DLL for the service.
///
/// SAFETY: the signature is fixed by the SCM ABI; the arguments are not used.
#[no_mangle]
pub extern "system" fn ServiceMain(_argc: u32, _argv: *mut *mut u16) {
    log::info!("running inside svchost service host");
    if let Err(err) = super::run_service(Vec::<OsString>::new()) {
        log::error!("svchost host exited: {err}");
    }
}