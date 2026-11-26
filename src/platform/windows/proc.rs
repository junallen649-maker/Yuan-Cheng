//! Process termination without spawning `taskkill.exe`.
//!
//! The uninstall path used to shell out to `taskkill /F /IM <app>.exe`, which is both a
//! command line a HIPS rule can score and a child process that may outlive the installer.
//! This walks the Toolhelp snapshot and terminates matching images in-process instead.

use hbb_common::log;

use crate::platform::win_api::Handle;

const TH32CS_SNAPPROCESS: u32 = 0x0000_0002;
const PROCESS_TERMINATE: u32 = 0x0001;
const MAX_PATH: usize = 260;

#[repr(C)]
struct ProcessEntry32W {
    dw_size: u32,
    cnt_usage: u32,
    th32_process_id: u32,
    th32_default_heap_id: usize,
    th32_module_id: u32,
    cnt_threads: u32,
    th32_parent_process_id: u32,
    pc_pri_class_base: i32,
    dw_flags: u32,
    sz_exe_file: [u16; MAX_PATH],
}

#[link(name = "kernel32")]
extern "system" {
    fn CreateToolhelp32Snapshot(flags: u32, pid: u32) -> Handle;
    fn Process32FirstW(snapshot: Handle, entry: *mut ProcessEntry32W) -> i32;
    fn Process32NextW(snapshot: Handle, entry: *mut ProcessEntry32W) -> i32;
    fn OpenProcess(access: u32, inherit: i32, pid: u32) -> Handle;
    fn TerminateProcess(process: Handle, code: u32) -> i32;
    fn CloseHandle(handle: Handle) -> i32;
    fn GetCurrentProcessId() -> u32;
}

fn invalid(handle: Handle) -> bool {
    handle.is_null() || handle as isize == -1
}

/// Terminate every process whose image name matches `exe_name` (ASCII case-insensitive),
/// except the caller. Returns how many were terminated.
pub fn kill_by_name(exe_name: &str) -> usize {
    let target = exe_name.to_ascii_lowercase();
    let self_pid = unsafe { GetCurrentProcessId() };
    let snapshot = unsafe { CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0) };
    if invalid(snapshot) {
        log::warn!("CreateToolhelp32Snapshot failed");
        return 0;
    }
    let mut entry = ProcessEntry32W {
        dw_size: std::mem::size_of::<ProcessEntry32W>() as u32,
        cnt_usage: 0,
        th32_process_id: 0,
        th32_default_heap_id: 0,
        th32_module_id: 0,
        cnt_threads: 0,
        th32_parent_process_id: 0,
        pc_pri_class_base: 0,
        dw_flags: 0,
        sz_exe_file: [0; MAX_PATH],
    };
    let mut killed = 0usize;
    let mut more = unsafe { Process32FirstW(snapshot, &mut entry) } != 0;
    while more {
        if entry.th32_process_id != self_pid {
            let end = entry
                .sz_exe_file
                .iter()
                .position(|c| *c == 0)
                .unwrap_or(MAX_PATH);
            let name = String::from_utf16_lossy(&entry.sz_exe_file[..end]);
            if name.to_ascii_lowercase() == target {
                let process =
                    unsafe { OpenProcess(PROCESS_TERMINATE, 0, entry.th32_process_id) };
                if !process.is_null() {
                    if unsafe { TerminateProcess(process, 1) } != 0 {
                        killed += 1;
                    }
                    unsafe { CloseHandle(process) };
                }
            }
        }
        entry.dw_size = std::mem::size_of::<ProcessEntry32W>() as u32;
        more = unsafe { Process32NextW(snapshot, &mut entry) } != 0;
    }
    unsafe { CloseHandle(snapshot) };
    log::info!("terminated {} process(es) named {}", killed, exe_name);
    killed
}