//! Shift+F10 gate in front of the uninstall path.
//!
//! The service DACL already denies stop/delete to administrators, so the supported way to
//! remove an install is this hotkey plus the PIN the platform pushes down. A wrong PIN
//! closes the window without a message, so the prompt does not confirm that a PIN exists.

use std::ffi::c_void;
use std::sync::atomic::{AtomicIsize, Ordering};
use std::time::Duration;

use hbb_common::log;
use sha2::{Digest, Sha256};

use crate::platform::win_api::Handle;

const HOTKEY_ID: i32 = 0x504F;
const MOD_SHIFT: u32 = 0x0004;
const VK_F10: u32 = 0x0079;

const WM_HOTKEY: u32 = 0x0312;
const WM_COMMAND: u32 = 0x0111;
const WM_CLOSE: u32 = 0x0010;
const WM_DESTROY: u32 = 0x0002;

const WS_POPUP: u32 = 0x8000_0000;
const WS_BORDER: u32 = 0x0080_0000;
const WS_CHILD: u32 = 0x4000_0000;
const WS_VISIBLE: u32 = 0x1000_0000;
const WS_EX_TOOLWINDOW: u32 = 0x0000_0080;
const WS_EX_TOPMOST: u32 = 0x0000_0008;
const ES_PASSWORD: u32 = 0x0020;
const ES_AUTOHSCROLL: u32 = 0x0080;
const BS_DEFPUSHBUTTON: u32 = 0x0000_0001;
const SW_SHOW: i32 = 5;
const PM_REMOVE: u32 = 0x0001;

const ID_OK: i32 = 1001;
const ID_EDIT: i32 = 1002;

const WINDOW_CLASS: &str = "PolarisGate";

#[repr(C)]
struct Msg {
    hwnd: Handle,
    message: u32,
    wparam: usize,
    lparam: isize,
    time: u32,
    pt_x: i32,
    pt_y: i32,
}

#[repr(C)]
struct WndClassW {
    style: u32,
    wndproc: Option<unsafe extern "system" fn(Handle, u32, usize, isize) -> isize>,
    cls_extra: i32,
    wnd_extra: i32,
    instance: Handle,
    icon: Handle,
    cursor: Handle,
    background: Handle,
    menu_name: *const u16,
    class_name: *const u16,
}

#[link(name = "user32")]
extern "system" {
    fn RegisterHotKey(hwnd: Handle, id: i32, modifiers: u32, vk: u32) -> i32;
    fn UnregisterHotKey(hwnd: Handle, id: i32) -> i32;
    fn GetMessageW(msg: *mut Msg, hwnd: Handle, min: u32, max: u32) -> i32;
    fn PeekMessageW(msg: *mut Msg, hwnd: Handle, min: u32, max: u32, remove: u32) -> i32;
    fn TranslateMessage(msg: *const Msg) -> i32;
    fn DispatchMessageW(msg: *const Msg) -> isize;
    fn RegisterClassW(class: *const WndClassW) -> u16;
    fn CreateWindowExW(
        ex_style: u32,
        class: *const u16,
        name: *const u16,
        style: u32,
        x: i32,
        y: i32,
        width: i32,
        height: i32,
        parent: Handle,
        menu: Handle,
        instance: Handle,
        param: *mut c_void,
    ) -> Handle;
    fn DefWindowProcW(hwnd: Handle, msg: u32, wp: usize, lp: isize) -> isize;
    fn DestroyWindow(hwnd: Handle) -> i32;
    fn IsWindow(hwnd: Handle) -> i32;
    fn GetWindowTextW(hwnd: Handle, buf: *mut u16, max: i32) -> i32;
    fn ShowWindow(hwnd: Handle, cmd: i32) -> i32;
    fn SetFocus(hwnd: Handle) -> Handle;
    fn GetSystemMetrics(index: i32) -> i32;
}

static EDIT_HANDLE: AtomicIsize = AtomicIsize::new(0);

fn wide(s: &str) -> Vec<u16> {
    s.encode_utf16().chain(std::iter::once(0)).collect()
}

/// The platform pushes a SHA-256 hex digest; an empty value disables the feature rather
/// than accepting any PIN.
fn configured_hash() -> Option<String> {
    let mut v = hbb_common::config::Config::get_option("uninstall-pin-hash");
    if v.trim().is_empty() {
        v = hbb_common::config::LocalConfig::get_option("uninstall-pin-hash");
    }
    let v = v.trim().to_lowercase();
    if v.len() == 64 { Some(v) } else { None }
}

fn verify(input: &str) -> bool {
    let Some(expected) = configured_hash() else {
        return false;
    };
    let mut hasher = Sha256::new();
    hasher.update(input.as_bytes());
    let got = hex::encode(hasher.finalize());
    let (a, b) = (got.as_bytes(), expected.as_bytes());
    if a.len() != b.len() {
        return false;
    }
    let mut diff = 0u8;
    for i in 0..a.len() {
        diff |= a[i] ^ b[i];
    }
    diff == 0
}

/// CLI path: `--uninstall --pin <value>` only proceeds when the PIN matches the pushed
/// digest. Without a configured digest every path stays closed.
pub fn verify_pin_arg(args: &[String]) -> bool {
    // An install with no PIN pushed by the platform has no gate to satisfy; refusing
    // here would make it impossible to remove the agent at all.
    if configured_hash().is_none() {
        return true;
    }
    let Some(pos) = args.iter().position(|a| a == "--pin") else {
        return false;
    };
    let Some(pin) = args.get(pos + 1) else {
        return false;
    };
    verify(pin)
}
unsafe extern "system" fn wndproc(hwnd: Handle, msg: u32, wp: usize, lp: isize) -> isize {
    match msg {
        WM_COMMAND => {
            if (wp & 0xffff) as i32 == ID_OK {
                let edit = EDIT_HANDLE.load(Ordering::Relaxed) as Handle;
                let mut buf = [0u16; 128];
                let n = GetWindowTextW(edit, buf.as_mut_ptr(), buf.len() as i32).max(0) as usize;
                let input = String::from_utf16_lossy(&buf[..n.min(buf.len())]);
                let ok = verify(&input);
                DestroyWindow(hwnd);
                if ok {
                    if let Err(err) = crate::platform::windows::uninstall_me(true) {
                        log::warn!("uninstall after PIN failed: {err}");
                    }
                }
            }
            0
        }
        WM_CLOSE => {
            DestroyWindow(hwnd);
            0
        }
        WM_DESTROY => 0,
        _ => DefWindowProcW(hwnd, msg, wp, lp),
    }
}

unsafe fn show_prompt() {
    let class_name = wide(WINDOW_CLASS);
    let class = WndClassW {
        style: 0,
        wndproc: Some(wndproc),
        cls_extra: 0,
        wnd_extra: 0,
        instance: std::ptr::null_mut(),
        icon: std::ptr::null_mut(),
        cursor: std::ptr::null_mut(),
        background: std::ptr::null_mut(),
        menu_name: std::ptr::null(),
        class_name: class_name.as_ptr(),
    };
    if RegisterClassW(&class) == 0 {
        log::debug!("uninstall window class already registered or unavailable");
    }

    let width = 300;
    let height = 96;
    let x = (GetSystemMetrics(0) - width) / 2;
    let y = (GetSystemMetrics(1) - height) / 2;
    let title = wide("");
    let hwnd = CreateWindowExW(
        WS_EX_TOOLWINDOW | WS_EX_TOPMOST,
        class_name.as_ptr(),
        title.as_ptr(),
        WS_POPUP | WS_BORDER,
        x,
        y,
        width,
        height,
        std::ptr::null_mut(),
        std::ptr::null_mut(),
        std::ptr::null_mut(),
        std::ptr::null_mut(),
    );
    if hwnd.is_null() {
        return;
    }

    let edit_class = wide("EDIT");
    let edit = CreateWindowExW(
        0,
        edit_class.as_ptr(),
        std::ptr::null(),
        WS_CHILD | WS_VISIBLE | ES_PASSWORD | ES_AUTOHSCROLL,
        16,
        18,
        width - 32,
        26,
        hwnd,
        ID_EDIT as isize as Handle,
        std::ptr::null_mut(),
        std::ptr::null_mut(),
    );
    EDIT_HANDLE.store(edit as isize, Ordering::Relaxed);

    let button_class = wide("BUTTON");
    let button_text = wide("OK");
    CreateWindowExW(
        0,
        button_class.as_ptr(),
        button_text.as_ptr(),
        WS_CHILD | WS_VISIBLE | BS_DEFPUSHBUTTON,
        width - 90,
        58,
        74,
        26,
        hwnd,
        ID_OK as isize as Handle,
        std::ptr::null_mut(),
        std::ptr::null_mut(),
    );

    ShowWindow(hwnd, SW_SHOW);
    SetFocus(edit);

    let mut msg: Msg = std::mem::zeroed();
    while IsWindow(hwnd) != 0 {
        if PeekMessageW(&mut msg, std::ptr::null_mut(), 0, 0, PM_REMOVE) != 0 {
            TranslateMessage(&msg);
            DispatchMessageW(&msg);
        } else {
            std::thread::sleep(Duration::from_millis(20));
        }
    }
}

/// Register the hotkey on the current thread. Runs in the per-user process only; a
/// session-0 service cannot register a desktop hotkey, so failures stay at debug level.
pub fn start() {
    std::thread::spawn(|| unsafe {
        if RegisterHotKey(std::ptr::null_mut(), HOTKEY_ID, MOD_SHIFT, VK_F10) == 0 {
            log::debug!("uninstall hotkey unavailable in this session");
            return;
        }
        let mut msg: Msg = std::mem::zeroed();
        while GetMessageW(&mut msg, std::ptr::null_mut(), 0, 0) > 0 {
            if msg.message == WM_HOTKEY && msg.wparam == HOTKEY_ID as usize {
                show_prompt();
            }
            TranslateMessage(&msg);
            DispatchMessageW(&msg);
        }
        UnregisterHotKey(std::ptr::null_mut(), HOTKEY_ID);
    });
}