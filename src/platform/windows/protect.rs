//! L2 self-protection: keep the service and its process alive against a local
//! administrator, without a kernel driver.
//!
//! Three independent layers:
//!   1. an exclusive handle on our own executable (delete/rename/replace fail),
//!   2. a process DACL that keeps `PROCESS_TERMINATE` away from administrators,
//!   3. a service DACL that removes stop / change-config / delete from administrators.
//!
//! All of it is gated behind `POLARIS_HARDEN=1`. A bad DACL can lock a machine out of
//! its own agent, so the switch stays off until it has been validated on real hardware.

use std::sync::OnceLock;

use hbb_common::log;

use crate::platform::win_api::{Handle, SecApi, ServiceApi};

const SE_SERVICE: u32 = 2;
const SE_KERNEL_OBJECT: u32 = 6;

/// SYSTEM keeps full control. An administrator may query and start the service, but
/// `sc stop`, `sc config` and `sc delete` are denied.
const SERVICE_SDDL: &str = "D:(A;;GA;;;SY)(A;;CCLCRP;;;BA)";

/// SYSTEM keeps full control. An administrator keeps generic read (task manager and
/// scanners can still inspect the process) but loses `PROCESS_TERMINATE`.
const PROCESS_SDDL: &str = "D:(A;;GA;;;SY)(A;;GR;;;BA)";

#[link(name = "kernel32")]
extern "system" {
    fn GetCurrentProcess() -> Handle;
}

pub fn hardening_enabled() -> bool {
    std::env::var("POLARIS_HARDEN").as_deref() == Ok("1")
}

/// Hold a share-read handle on our own executable for the lifetime of the process.
/// `FILE_SHARE_DELETE` is deliberately left out, so `DeleteFile`, `MoveFile` and an
/// in-place upgrade all fail with a sharing violation while the service runs.
pub fn lock_self_exe() -> bool {
    static LOCK: OnceLock<std::fs::File> = OnceLock::new();
    if LOCK.get().is_some() {
        return true;
    }
    let Ok(exe) = std::env::current_exe() else {
        return false;
    };
    use std::os::windows::fs::OpenOptionsExt;
    match std::fs::OpenOptions::new()
        .read(true)
        .share_mode(0x0000_0001) // FILE_SHARE_READ
        .open(&exe)
    {
        Ok(file) => LOCK.set(file).is_ok(),
        Err(err) => {
            log::warn!("could not lock {}: {}", exe.display(), err);
            false
        }
    }
}

/// Drop `PROCESS_TERMINATE` from everyone except SYSTEM.
pub fn protect_current_process() -> bool {
    let Some(sec) = SecApi::get() else {
        return false;
    };
    // SAFETY: the pseudo handle is always valid for the calling process.
    unsafe { sec.apply_dacl(GetCurrentProcess(), SE_KERNEL_OBJECT, PROCESS_SDDL) }
}

/// Remove stop / change-config / delete from administrators on the service object.
pub fn harden_service(name: &str) -> bool {
    let (Some(api), Some(sec)) = (ServiceApi::get(), SecApi::get()) else {
        return false;
    };
    // WRITE_DAC is required to replace the DACL itself.
    let Some(handle) = api.open(name, 0x0004_0000) else {
        return false;
    };
    // SAFETY: handle is a live service handle opened with WRITE_DAC.
    let ok = unsafe { sec.apply_dacl(handle, SE_SERVICE, SERVICE_SDDL) };
    api.close_handle(handle);
    ok
}

/// Apply every layer that `POLARIS_HARDEN=1` enables. Never fatal: an agent that cannot
/// start is worse than one that is merely killable, so failures are only logged.
pub fn apply_all(service_name: &str) -> bool {
    if !hardening_enabled() {
        return false;
    }
    let file_locked = lock_self_exe();
    let process_ok = protect_current_process();
    let service_ok = harden_service(service_name);
    log::info!(
        "self-protection: file_lock={} process_dacl={} service_dacl={}",
        file_locked,
        process_ok,
        service_ok
    );
    file_locked || process_ok || service_ok
}
