use std::{
    collections::{HashMap, HashSet},
    sync::{Arc, Mutex},
    time::Duration,
};

#[cfg(not(any(target_os = "ios")))]
use crate::{common::API_LOG_INTERVAL, ui_interface::get_builtin_option, Connection};
#[cfg(not(any(target_os = "ios")))]
use hbb_common::{
    bytes::Bytes,
    config::{self, Config, LocalConfig, CONNECT_TIMEOUT, RENDEZVOUS_PORT},
    log, password_security,
    rendezvous_proto::{register_pk_response, rendezvous_message, RegisterPk, RendezvousMessage},
    socket_client,
    tokio::{self, sync::broadcast, time::Instant},
};
#[cfg(target_os = "ios")]
use hbb_common::{
    config::{self, Config, LocalConfig},
    log, password_security, tokio,
};
use base::config::keys;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};

const TIME_HEARTBEAT: Duration = Duration::from_secs(15);
const UPLOAD_SYSINFO_TIMEOUT: Duration = Duration::from_secs(120);
const TIME_CONN: Duration = Duration::from_secs(3);

#[cfg(not(any(target_os = "ios")))]
lazy_static::lazy_static! {
    static ref SENDER : Mutex<broadcast::Sender<Vec<i32>>> = Mutex::new(start_hbbs_sync());
    static ref PRO: Arc<Mutex<bool>> = Default::default();
    static ref IN_FLIGHT_CMDS: Mutex<HashSet<u64>> = Default::default();
}

#[cfg(not(any(target_os = "ios")))]
pub fn start() {
    let _sender = SENDER.lock().unwrap();
}

#[cfg(not(target_os = "ios"))]
pub fn signal_receiver() -> broadcast::Receiver<Vec<i32>> {
    SENDER.lock().unwrap().subscribe()
}

#[cfg(not(any(target_os = "ios")))]
fn start_hbbs_sync() -> broadcast::Sender<Vec<i32>> {
    let (tx, _rx) = broadcast::channel::<Vec<i32>>(16);
    std::thread::spawn(move || start_hbbs_sync_async());
    return tx;
}

#[derive(Debug, Serialize, Deserialize)]
pub struct StrategyOptions {
    #[serde(default, skip_serializing_if = "HashMap::is_empty")]
    pub config_options: HashMap<String, String>,
    #[serde(default, skip_serializing_if = "HashMap::is_empty")]
    pub extra: HashMap<String, String>,
}

struct InfoUploaded {
    uploaded: bool,
    url: String,
    last_uploaded: Option<Instant>,
    id: String,
    username: Option<String>,
}

impl Default for InfoUploaded {
    fn default() -> Self {
        Self {
            uploaded: false,
            url: "".to_owned(),
            last_uploaded: None,
            id: "".to_owned(),
            username: None,
        }
    }
}

impl InfoUploaded {
    fn uploaded(url: String, id: String, username: String) -> Self {
        Self {
            uploaded: true,
            url,
            last_uploaded: None,
            id,
            username: Some(username),
        }
    }
}

#[cfg(not(any(target_os = "ios")))]
#[tokio::main(flavor = "current_thread")]
async fn start_hbbs_sync_async() {
    let mut interval = crate::rustdesk_interval(tokio::time::interval_at(
        Instant::now() + TIME_CONN,
        TIME_CONN,
    ));
    let mut last_sent: Option<Instant> = None;
    let mut info_uploaded = InfoUploaded::default();
    let mut sysinfo_ver = "".to_owned();
    loop {
        tokio::select! {
            _ = interval.tick() => {
                let url = heartbeat_url();
                let id = Config::get_id();
                if url.is_empty() {
                    *PRO.lock().unwrap() = false;
                    continue;
                }
                if current_client_role() == "agent"
                    && config::option2bool("stop-service", &Config::get_option("stop-service"))
                {
                    continue;
                }
                let conns = Connection::alive_conns();
                if info_uploaded.uploaded && (url != info_uploaded.url || id != info_uploaded.id) {
                    info_uploaded.uploaded = false;
                    *PRO.lock().unwrap() = false;
                }
                // For Windows:
                // We can't skip uploading sysinfo when the username is empty, because the username may
                // always be empty before login. We also need to upload the other sysinfo info.
                //
                // https://github.com/rustdesk/rustdesk/discussions/8031
                // We still need to check the username after uploading sysinfo, because
                // 1. The username may be empty when logining in, and it can be fetched after a while.
                //    In this case, we need to upload sysinfo again.
                // 2. The username may be changed after uploading sysinfo, and we need to upload sysinfo again.
                //
                // The Windows session will switch to the last user session before the restart,
                // so it may be able to get the username before login.
                // But strangely, sometimes we can get the username before login,
                // we may not be able to get the username before login after the next restart.
                let mut v = crate::get_sysinfo();
                let sys_username = v["username"].as_str().unwrap_or_default().to_string();
                // Though the username comparison is only necessary on Windows,
                // we still keep the comparison on other platforms for consistency.
                let need_upload = (!info_uploaded.uploaded || info_uploaded.username.as_ref() != Some(&sys_username)) &&
                    info_uploaded.last_uploaded.map(|x| x.elapsed() >= UPLOAD_SYSINFO_TIMEOUT).unwrap_or(true);
                if need_upload {
                    v["version"] = json!(crate::VERSION);
                    v["id"] = json!(id);
                    v["uuid"] = json!(crate::encode64(hbb_common::get_uuid()));
                    v["client_role"] = json!(current_client_role());
                    let local_ips = collect_local_ips();
                    if !local_ips.is_empty() {
                        v["local_ips"] = json!(local_ips);
                    }
                    if !config::is_outgoing_only() {
                        v["temp_password"] = json!(password_security::temporary_password());
                        v["has_permanent_password"] = json!(Config::has_permanent_password());
                    }
                    let school_code = Config::get_option("school-code");
                    if !school_code.is_empty() {
                        v["school_code"] = json!(school_code);
                    }
                    let enroll_token = Config::get_option("enroll-token");
                    if !enroll_token.is_empty() {
                        v["enroll_token"] = json!(enroll_token);
                    }
                    let ab_name = Config::get_option(keys::OPTION_PRESET_ADDRESS_BOOK_NAME);
                    if !ab_name.is_empty() {
                        v[keys::OPTION_PRESET_ADDRESS_BOOK_NAME] = json!(ab_name);
                    }
                    let ab_tag = Config::get_option(keys::OPTION_PRESET_ADDRESS_BOOK_TAG);
                    if !ab_tag.is_empty() {
                        v[keys::OPTION_PRESET_ADDRESS_BOOK_TAG] = json!(ab_tag);
                    }
                    let ab_alias = Config::get_option(keys::OPTION_PRESET_ADDRESS_BOOK_ALIAS);
                    if !ab_alias.is_empty() {
                        v[keys::OPTION_PRESET_ADDRESS_BOOK_ALIAS] = json!(ab_alias);
                    }
                    let ab_password = Config::get_option(keys::OPTION_PRESET_ADDRESS_BOOK_PASSWORD);
                    if !ab_password.is_empty() {
                        v[keys::OPTION_PRESET_ADDRESS_BOOK_PASSWORD] = json!(ab_password);
                    }
                    let ab_note = Config::get_option(keys::OPTION_PRESET_ADDRESS_BOOK_NOTE);
                    if !ab_note.is_empty() {
                        v[keys::OPTION_PRESET_ADDRESS_BOOK_NOTE] = json!(ab_note);
                    }
                    let username = get_builtin_option(keys::OPTION_PRESET_USERNAME);
                    if !username.is_empty() {
                        v[keys::OPTION_PRESET_USERNAME] = json!(username);
                    }
                    let strategy_name = get_builtin_option(keys::OPTION_PRESET_STRATEGY_NAME);
                    if !strategy_name.is_empty() {
                        v[keys::OPTION_PRESET_STRATEGY_NAME] = json!(strategy_name);
                    }
                    let device_group_name = get_builtin_option(keys::OPTION_PRESET_DEVICE_GROUP_NAME);
                    if !device_group_name.is_empty() {
                        v[keys::OPTION_PRESET_DEVICE_GROUP_NAME] = json!(device_group_name);
                    }
                    let device_username = Config::get_option(keys::OPTION_PRESET_DEVICE_USERNAME);
                    if !device_username.is_empty() {
                        v["username"] = json!(device_username);
                    }
                    let device_name = Config::get_option(keys::OPTION_PRESET_DEVICE_NAME);
                    if !device_name.is_empty() {
                        v["hostname"] = json!(device_name);
                    }
                    let note = Config::get_option(keys::OPTION_PRESET_NOTE);
                    if !note.is_empty() {
                        v[keys::OPTION_PRESET_NOTE] = json!(note);
                    }
                    let v = v.to_string();
                    let mut hash = "".to_owned();
                    if crate::is_public(&url) {
                        use sha2::{Digest, Sha256};
                        let mut hasher = Sha256::new();
                        hasher.update(url.as_bytes());
                        hasher.update(&v.as_bytes());
                        let res = hasher.finalize();
                        hash = hbb_common::base64::encode(&res[..]);
                        let old_hash = config::Status::get("sysinfo_hash");
                        let ver = config::Status::get("sysinfo_ver"); // sysinfo_ver is the version of sysinfo on server's side
                        if hash == old_hash {
                            // When the api doesn't exist, Ok("") will be returned in test.
                            let samever = match crate::post_request(url.replace("heartbeat", "sysinfo_ver"), "".to_owned(), "").await {
                                Ok(x)  => {
                                    sysinfo_ver = x.clone();
                                    *PRO.lock().unwrap() = true;
                                    x == ver
                                }
                                _ => {
                                    false // to make sure Pro can be assigned in below post for old
                                            // hbbs pro not supporting sysinfo_ver, use false for ensuring
                                }
                            };
                            if samever {
                                info_uploaded = InfoUploaded::uploaded(url.clone(), id.clone(), sys_username);
                                log::info!("sysinfo not changed, skip upload");
                                continue;
                            }
                        }
                    }
                    match crate::post_request(url.replace("heartbeat", "sysinfo"), v, "").await {
                        Ok(x)  => {
                            if x == "SYSINFO_UPDATED" {
                                info_uploaded = InfoUploaded::uploaded(url.clone(), id.clone(), sys_username);
                                log::info!("sysinfo updated");
                                if !hash.is_empty() {
                                    config::Status::set("sysinfo_hash", hash);
                                    config::Status::set("sysinfo_ver", sysinfo_ver.clone());
                                }
                                *PRO.lock().unwrap() = true;
                            } else if x == "ID_NOT_FOUND" {
                                info_uploaded.last_uploaded = None; // next heartbeat will upload sysinfo again
                            } else {
                                info_uploaded.last_uploaded = Some(Instant::now());
                            }
                        }
                        _ => {
                            info_uploaded.last_uploaded = Some(Instant::now());
                        }
                    }
                }
                if conns.is_empty() && last_sent.map(|x| x.elapsed() < TIME_HEARTBEAT).unwrap_or(false) {
                    continue;
                }
                last_sent = Some(Instant::now());
                let mut v = Value::default();
                v["id"] = json!(id);
                v["uuid"] = json!(crate::encode64(hbb_common::get_uuid()));
                v["ver"] = json!(hbb_common::get_version_number(crate::VERSION));
                v["client_role"] = json!(current_client_role());
                let bound_user = get_bound_user_name();
                if !bound_user.is_empty() {
                    v["bound_user"] = json!(bound_user);
                }
                let local_ips = collect_local_ips();
                if !local_ips.is_empty() {
                    v["local_ips"] = json!(local_ips);
                }
                if !config::is_outgoing_only() {
                    v["temp_password"] = json!(password_security::temporary_password());
                    v["has_permanent_password"] = json!(Config::has_permanent_password());
                }
                if !conns.is_empty() {
                    v["conns"] = json!(conns);
                }
                let modified_at = LocalConfig::get_option("strategy_timestamp").parse::<i64>().unwrap_or(0);
                v["modified_at"] = json!(modified_at);
                let response = crate::post_request(url.clone(), v.to_string(), "").await;
                if let Err(err) = &response {
                    hbb_common::throttled_log!(API_LOG_INTERVAL, warn, "Heartbeat failed: {err:?}");
                }
                if let Ok(s) = response {
                    if let Ok(mut rsp) = serde_json::from_str::<HashMap::<&str, Value>>(&s) {
                        if rsp.remove("sysinfo").is_some() {
                            info_uploaded.uploaded = false;
                            config::Status::set("sysinfo_hash", "".to_owned());
                            log::info!("sysinfo required to forcely update");
                        }
                        if let Some(conns)  = rsp.remove("disconnect") {
                                if let Ok(conns) = serde_json::from_value::<Vec<i32>>(conns) {
                                    SENDER.lock().unwrap().send(conns).ok();
                                }
                        }
                        if let Some(rsp_modified_at) = rsp.remove("modified_at") {
                            if let Ok(rsp_modified_at) = serde_json::from_value::<i64>(rsp_modified_at) {
                                if rsp_modified_at != modified_at {
                                    LocalConfig::set_option("strategy_timestamp".to_string(), rsp_modified_at.to_string());
                                }
                            }
                        }
                        if let Some(strategy) = rsp.remove("strategy") {
                            if let Ok(strategy) = serde_json::from_value::<StrategyOptions>(strategy) {
                                log::info!("strategy updated");
                                handle_config_options(strategy.config_options);
                            }
                        }
                        if let Some(commands) = rsp.remove("commands") {
                            if let Ok(cmds) = serde_json::from_value::<Vec<RemoteCommand>>(commands) {
                                handle_remote_commands(&url, &id, cmds);
                            }
                        }
                    }
                }
            }
        }
    }
}

#[cfg(not(any(target_os = "ios")))]
#[derive(Debug, Deserialize)]
struct RemoteCommand {
    id: u64,
    #[serde(rename = "type")]
    cmd_type: String,
    #[serde(default)]
    payload: HashMap<String, String>,
}

#[cfg(not(any(target_os = "ios")))]
fn handle_remote_commands(heartbeat_url_str: &str, device_id: &str, cmds: Vec<RemoteCommand>) {
    let ack_url = heartbeat_url_str.replace("/api/heartbeat", "/api/command/ack");
    for cmd in cmds {
        {
            let mut guard = IN_FLIGHT_CMDS.lock().unwrap();
            if !guard.insert(cmd.id) {
                continue;
            }
        }
        let ack_url = ack_url.clone();
        let dev_id = device_id.to_owned();
        tokio::spawn(async move {
            execute_remote_command(&ack_url, &dev_id, cmd).await;
        });
    }
}

#[cfg(not(any(target_os = "ios")))]
async fn execute_remote_command(ack_url: &str, device_id: &str, cmd: RemoteCommand) {
    let cmd_id = cmd.id;
    let mut ack = json!({
        "cmd_id": cmd_id,
        "device_id": device_id,
        "status": "failed",
        "message": "",
    });

    match cmd.cmd_type.as_str() {
        "set_temp_password" => {
            let pw = cmd.payload.get("password").cloned().unwrap_or_default();
            if pw.trim().len() < 4 {
                ack["message"] = json!("临时密码长度不能少于4位");
            } else {
                password_security::set_temporary_password(&pw);
                ack["status"] = json!("succeeded");
                ack["temp_password"] = json!(password_security::temporary_password());
                ack["message"] = json!("临时密码已更新");
            }
        }
        "refresh_temp_password" => {
            password_security::update_temporary_password();
            ack["status"] = json!("succeeded");
            ack["temp_password"] = json!(password_security::temporary_password());
            ack["message"] = json!("已刷新随机临时密码");
        }
        "set_permanent_password" => {
            let pw = cmd.payload.get("password").cloned().unwrap_or_default();
            if Config::set_permanent_password(pw.trim()) {
                ack["status"] = json!("succeeded");
                ack["has_permanent_password"] = json!(Config::has_permanent_password());
                ack["message"] = json!(if pw.trim().is_empty() { "固定密码已清除" } else { "固定密码已更新" });
            } else {
                ack["message"] = json!("设置固定密码失败");
            }
        }
        "change_id" => {
            let new_id = cmd.payload.get("new_id").cloned().unwrap_or_default();
            let old_id = Config::get_id();
            if new_id == old_id {
                ack["status"] = json!("succeeded");
                ack["old_id"] = json!(old_id);
                ack["new_id"] = json!(new_id);
                ack["message"] = json!("连接代码已是目标值");
            } else {
                match change_id_remote(&old_id, &new_id).await {
                    Ok(()) => {
                        ack["status"] = json!("succeeded");
                        ack["old_id"] = json!(old_id);
                        ack["new_id"] = json!(new_id);
                        ack["temp_password"] = json!(password_security::temporary_password());
                        ack["message"] = json!("连接代码已更改并重新注册");
                    }
                    Err(err) => {
                        ack["message"] = json!(err);
                    }
                }
            }
        }
        other => {
            ack["message"] = json!(format!("未知指令类型: {other}"));
        }
    }

    if let Err(err) = crate::post_request(ack_url.to_owned(), ack.to_string(), "").await {
        log::warn!("command ack failed for cmd {}: {:?}", cmd_id, err);
    }
    IN_FLIGHT_CMDS.lock().unwrap().remove(&cmd_id);
}

#[cfg(not(any(target_os = "ios")))]
async fn change_id_remote(old_id: &str, new_id: &str) -> Result<(), String> {
    if !hbb_common::is_valid_custom_id(new_id) {
        return Err("连接代码格式无效，需以字母开头且为6~16位字母/数字/下划线/短横线".to_owned());
    }
    let uuid = Bytes::from(hbb_common::get_uuid());
    if uuid.is_empty() {
        return Err("获取本机 UUID 失败".to_owned());
    }
    let servers = Config::get_rendezvous_servers();
    if servers.is_empty() {
        return Err("未配置信令服务器".to_owned());
    }
    for rv in servers {
        let addr = crate::check_port(rv.clone(), RENDEZVOUS_PORT);
        let mut socket = socket_client::connect_tcp(addr, CONNECT_TIMEOUT)
            .await
            .map_err(|e| format!("连接信令服务器失败: {e}"))?;
        let mut msg_out = RendezvousMessage::new();
        msg_out.set_register_pk(RegisterPk {
            old_id: old_id.to_owned(),
            id: new_id.to_owned(),
            uuid: uuid.clone(),
            ..Default::default()
        });
        socket
            .send(&msg_out)
            .await
            .map_err(|e| format!("发送换绑请求失败: {e}"))?;
        let Some(msg_in) = crate::common::get_next_nonkeyexchange_msg(&mut socket, None).await else {
            return Err("信令服务器未返回响应".to_owned());
        };
        if let Some(rendezvous_message::Union::RegisterPkResponse(rpr)) = msg_in.union {
            match rpr.result.enum_value() {
                Ok(register_pk_response::Result::OK) => {}
                Ok(register_pk_response::Result::ID_EXISTS) => {
                    return Err("目标连接代码已被占用 (ID_EXISTS)".to_owned());
                }
                Ok(register_pk_response::Result::TOO_FREQUENT) => {
                    return Err("更换连接代码过于频繁 (TOO_FREQUENT)".to_owned());
                }
                Ok(register_pk_response::Result::INVALID_ID_FORMAT) => {
                    return Err("信令服务器拒绝该 ID 格式".to_owned());
                }
                Ok(register_pk_response::Result::NOT_SUPPORT) => {
                    return Err("信令服务器不支持自定义 ID 换绑".to_owned());
                }
                other => {
                    return Err(format!("换绑被拒绝: {other:?}"));
                }
            }
        } else {
            return Err("收到非预期信令响应".to_owned());
        }
    }
    Config::set_key_confirmed(false);
    Config::set_id(new_id);
    crate::rendezvous_mediator::RendezvousMediator::restart();
    Ok(())
}

fn current_client_role() -> &'static str {
    if config::is_incoming_only() || Config::get_option("client-role") == "agent" {
        "agent"
    } else {
        "console"
    }
}

fn collect_local_ips() -> String {
    #[cfg(not(any(target_os = "android", target_os = "ios")))]
    {
        let mut ips = Vec::new();
        for iface in default_net::get_interfaces() {
            for ipv4 in &iface.ipv4 {
                if !ipv4.addr.is_loopback() && !ipv4.addr.is_link_local() {
                    let s = ipv4.addr.to_string();
                    if !ips.contains(&s) {
                        ips.push(s);
                    }
                }
            }
        }
        return ips.join(", ");
    }
    #[cfg(any(target_os = "android", target_os = "ios"))]
    String::new()
}

fn get_bound_user_name() -> String {
    let raw = LocalConfig::get_option("user_info");
    if raw.is_empty() {
        return String::new();
    }
    serde_json::from_str::<Value>(&raw)
        .ok()
        .and_then(|v| v.get("name").and_then(|n| n.as_str()).map(|s| s.to_owned()))
        .unwrap_or_default()
}

pub fn report_outgoing_session(
    peer_id: String,
    peer_name: String,
    peer_platform: String,
    peer_username: String,
    conn_type: i32,
    action: &'static str,
    nonce: String,
) {
    let api_server = crate::common::get_api_server(
        Config::get_option("api-server"),
        Config::get_option("custom-rendezvous-server"),
    );
    if api_server.is_empty() || crate::is_public(&api_server) {
        return;
    }
    let url = format!("{}/api/audit/outgoing", api_server);
    let my_id = Config::get_id();
    let uuid = crate::encode64(hbb_common::get_uuid());
    let local_ips = collect_local_ips();
    let bound_user = get_bound_user_name();
    let client_role = current_client_role();
    let version = crate::VERSION.to_owned();
    let hostname = crate::hostname();
    #[cfg(not(any(target_os = "android", target_os = "ios")))]
    let username = crate::platform::get_active_username();
    #[cfg(any(target_os = "android", target_os = "ios"))]
    let username = String::new();
    #[cfg(any(target_os = "android", target_os = "ios"))]
    let os = String::new();
    #[cfg(not(any(target_os = "android", target_os = "ios")))]
    let os = hbb_common::whoami::platform().to_string();

    tokio::spawn(async move {
        let body = json!({
            "id": my_id,
            "uuid": uuid,
            "client_role": client_role,
            "hostname": hostname,
            "username": username,
            "bound_user": bound_user,
            "local_ips": local_ips,
            "os": os,
            "version": version,
            "peer_id": peer_id,
            "peer_name": peer_name,
            "peer_platform": peer_platform,
            "peer_username": peer_username,
            "conn_type": conn_type,
            "action": action,
            "nonce": nonce,
        })
        .to_string();
        if let Err(err) = crate::post_request(url, body, "").await {
            log::trace!("report_outgoing_session failed: {err:?}");
        }
    });
}

fn heartbeat_url() -> String {
    let url = crate::common::get_api_server(
        Config::get_option("api-server"),
        Config::get_option("custom-rendezvous-server"),
    );
    if url.is_empty() || crate::is_public(&url) {
        return "".to_owned();
    }
    format!("{}/api/heartbeat", url)
}

fn handle_config_options(config_options: HashMap<String, String>) {
    let mut options = Config::get_options();
    let default_settings = config::DEFAULT_SETTINGS.read().unwrap().clone();
    config_options
        .iter()
        .map(|(k, v)| {
            // Priority: user config > default advanced options.
            // Only when default advanced options are also empty, remove user option (fallback to built-in default);
            // otherwise insert an empty value so user config remains present.
            if v.is_empty() && default_settings.get(k).map_or("", |v| v).is_empty() {
                options.remove(k);
            } else {
                options.insert(k.to_string(), v.to_string());
            }
        })
        .count();
    Config::set_options(options);
}

#[allow(unused)]
#[cfg(not(any(target_os = "ios")))]
pub fn is_pro() -> bool {
    PRO.lock().unwrap().clone()
}

// Fire-and-forget by design: the switch flow must not block on this POST.
// If the device clock is outside the server's accepted window, the server
// returns its current Unix time and this task re-signs and retries once.
#[cfg(feature = "flutter")]
#[cfg(not(any(target_os = "android", target_os = "ios")))]
pub fn register_switch_grant(switch_uuid: String) {
    tokio::spawn(async move {
        let api_server = crate::ui_interface::get_api_server();
        if api_server.is_empty() || crate::is_public(&api_server) {
            return;
        }
        use hbb_common::sodiumoxide::crypto::{hash::sha256, sign};
        let switch_code = crate::encode64(sha256::hash(switch_uuid.as_bytes()).0);
        let switch_code_verifier = switch_code_verifier(&switch_code);
        let timestamp = (hbb_common::get_time() / 1000).to_string();
        let id = Config::get_id();
        let kp = Config::get_key_pair();
        let Some(sk) = sign::SecretKey::from_slice(&kp.0) else {
            log::error!("Failed to register switch grant: no device key");
            return;
        };
        let url = format!("{}/api/switch-grant", api_server);
        let mut timestamp = timestamp;
        for attempt in 0..2 {
            let signature = sign::sign_detached(
                &switch_grant_signed_msg(&id, &switch_code_verifier, &timestamp),
                &sk,
            );
            let body = json!({
                "id": &id,
                "switch_code_verifier": &switch_code_verifier,
                "timestamp": &timestamp,
                "signature": crate::encode64(signature.to_bytes()),
            })
            .to_string();
            let response = match crate::post_request(url.clone(), body, "").await {
                Ok(response) => response,
                Err(e) => {
                    log::error!("Failed to register switch grant: {}", e);
                    return;
                }
            };
            let response = match serde_json::from_str::<Value>(&response) {
                Ok(response) => response,
                Err(e) => {
                    log::error!("Failed to register switch grant: invalid response: {}", e);
                    return;
                }
            };
            match response.get("accepted").and_then(Value::as_bool) {
                Some(true) => return,
                Some(false) => {}
                None => {
                    log::error!("Failed to register switch grant: missing accepted response");
                    return;
                }
            }
            let Some(server_time) = response["server_time"].as_i64() else {
                log::error!("Failed to register switch grant: rejected by server");
                return;
            };
            if attempt == 0 {
                log::warn!("Switch grant timestamp rejected, retrying with server time");
                timestamp = server_time.to_string();
            } else {
                log::error!("Failed to register switch grant after retrying with server time");
            }
        }
    });
}

#[cfg(feature = "flutter")]
#[cfg(not(any(target_os = "android", target_os = "ios")))]
fn switch_code_verifier(switch_code: &str) -> String {
    use hbb_common::sodiumoxide::crypto::hash::sha256;

    let prefix = b"switch-grant-verifier\0";
    let mut msg = Vec::with_capacity(prefix.len() + switch_code.len());
    msg.extend_from_slice(prefix);
    msg.extend_from_slice(switch_code.as_bytes());
    crate::encode64(sha256::hash(&msg).0)
}

#[cfg(feature = "flutter")]
#[cfg(not(any(target_os = "android", target_os = "ios")))]
fn switch_grant_signed_msg(id: &str, switch_code_verifier: &str, timestamp: &str) -> Vec<u8> {
    let mut msg =
        Vec::with_capacity(13 + id.len() + 1 + switch_code_verifier.len() + 1 + timestamp.len());
    msg.extend_from_slice(b"switch-grant\0");
    msg.extend_from_slice(id.as_bytes());
    msg.push(0);
    msg.extend_from_slice(switch_code_verifier.as_bytes());
    msg.push(0);
    msg.extend_from_slice(timestamp.as_bytes());
    msg
}

#[cfg(all(
    test,
    feature = "flutter",
    not(any(target_os = "android", target_os = "ios"))
))]
mod tests {
    use super::{switch_code_verifier, switch_grant_signed_msg};

    #[test]
    fn test_switch_code_verifier_is_not_raw_switch_code() {
        let switch_code = "code-abc";
        let verifier = switch_code_verifier(switch_code);
        assert_ne!(verifier, switch_code);
        assert_eq!(verifier, switch_code_verifier(switch_code));
        assert_eq!(
            verifier,
            "dMIn3uiPe77XodFB5IKi7PrKJ7l7+zVquNn0ObSaHQc="
        );
    }

    #[test]
    fn test_switch_grant_signed_msg_layout() {
        let expected: Vec<u8> = [
            &b"switch-grant\0"[..],
            b"id1",
            b"\0",
            b"c1",
            b"\0",
            b"1700000000",
        ]
        .concat();
        assert_eq!(switch_grant_signed_msg("id1", "c1", "1700000000"), expected);
    }
}
