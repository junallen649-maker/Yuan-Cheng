import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_hbb/common.dart';
import 'package:flutter_hbb/common/formatter/id_formatter.dart';
import 'package:flutter_hbb/common/widgets/animated_rotation_widget.dart';
import 'package:flutter_hbb/models/platform_model.dart';
import 'package:flutter_hbb/models/server_model.dart';
import 'package:get/get.dart';
import 'package:provider/provider.dart';

/// UU 远程风格 - 视图 C：远程协助主控与受控面板
class DeskRemoteAssistView extends StatefulWidget {
  const DeskRemoteAssistView({Key? key}) : super(key: key);

  @override
  State<DeskRemoteAssistView> createState() => _DeskRemoteAssistViewState();
}

class _DeskRemoteAssistViewState extends State<DeskRemoteAssistView> {
  final TextEditingController _partnerIdController = TextEditingController();
  final RxBool _obscurePassword = true.obs;
  final RxBool _allowRemoteControl = true.obs;
  final RxString _verifyMethod = 'temporary'.obs; // 'temporary' | 'permanent'

  @override
  void initState() {
    super.initState();
    _initServiceState();
  }

  void _initServiceState() {
    // 监听本地服务状态，同步允许协助开关
    final stopService = bind.mainGetLocalOption(key: 'stop-service');
    _allowRemoteControl.value = stopService != 'Y';
  }

  void _toggleAllowRemote(bool val) async {
    _allowRemoteControl.value = val;
    if (val) {
      await bind.mainSetLocalOption(key: 'stop-service', value: '');
      await start_service(true);
      showToast(translate('Service is running'));
    } else {
      await bind.mainSetLocalOption(key: 'stop-service', value: 'Y');
      await start_service(false);
      showToast(translate('Service is stopped'));
    }
  }

  void _copyAndShare(String id, String password) {
    final cleanId = formatID(id);
    final text = '【Polaris远程】我的设备ID：$cleanId，临时验证码：$password';
    Clipboard.setData(ClipboardData(text: text));
    showToast('已复制设备ID与验证码，可直接发送给伙伴');
  }

  void _onConnectPartner() {
    final id = _partnerIdController.text.trim().replaceAll(' ', '');
    if (id.isEmpty) {
      showToast('请输入伙伴的设备ID');
      return;
    }
    connect(context, id);
  }

  @override
  void dispose() {
    _partnerIdController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final cardBg = isDark ? const Color(0xFF1E293B) : Colors.white;
    final borderColor = isDark ? const Color(0xFF334155) : const Color(0xFFE2E8F0);
    final primaryColor = const Color(0xFF1E6FFF);

    return ChangeNotifierProvider.value(
      value: gFFI.serverModel,
      child: Consumer<ServerModel>(
        builder: (context, serverModel, child) {
          final myId = serverModel.serverId.text;
          final myPassword = serverModel.serverPasswd.text;
          final formattedId = formatID(myId);

          return SingleChildScrollView(
            padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 28),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // 页面大标题
                const Text(
                  '远程协助',
                  style: TextStyle(
                    fontSize: 24,
                    fontWeight: FontWeight.bold,
                    letterSpacing: 0.5,
                  ),
                ),
                const SizedBox(height: 20),

                // 卡片 1: 本设备受控面板
                Container(
                  width: double.infinity,
                  padding: const EdgeInsets.all(24),
                  decoration: BoxDecoration(
                    color: cardBg,
                    borderRadius: BorderRadius.circular(16),
                    border: Border.all(color: borderColor, width: 1),
                    boxShadow: [
                      BoxShadow(
                        color: Colors.black.withOpacity(isDark ? 0.2 : 0.04),
                        blurRadius: 12,
                        offset: const Offset(0, 4),
                      ),
                    ],
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      // 卡片标题栏：本设备 + 允许协助 Switch 开关
                      Row(
                        mainAxisAlignment: MainAxisAlignment.spaceBetween,
                        children: [
                          const Text(
                            '本设备',
                            style: TextStyle(
                              fontSize: 18,
                              fontWeight: FontWeight.bold,
                            ),
                          ),
                          Row(
                            children: [
                              Text(
                                '允许他人远程协助',
                                style: TextStyle(
                                  fontSize: 14,
                                  color: isDark ? Colors.grey[300] : const Color(0xFF475569),
                                ),
                              ),
                              const SizedBox(width: 10),
                              Obx(
                                () => Switch(
                                  value: _allowRemoteControl.value,
                                  activeColor: primaryColor,
                                  onChanged: _toggleAllowRemote,
                                ),
                              ),
                            ],
                          ),
                        ],
                      ),
                      const Divider(height: 32),

                      // 受控核心信息排版（三列网格布局：本设备ID、验证方式与密码、复制分享按钮）
                      LayoutBuilder(
                        builder: (context, constraints) {
                          return Row(
                            crossAxisAlignment: CrossAxisAlignment.center,
                            children: [
                              // 列 1: 本设备 ID
                              Expanded(
                                flex: 4,
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Text(
                                      '本设备ID',
                                      style: TextStyle(
                                        fontSize: 13,
                                        color: isDark ? Colors.grey[400] : const Color(0xFF64748B),
                                      ),
                                    ),
                                    const SizedBox(height: 8),
                                    InkWell(
                                      onTap: () {
                                        Clipboard.setData(ClipboardData(text: myId));
                                        showToast('设备ID已复制');
                                      },
                                      borderRadius: BorderRadius.circular(8),
                                      child: Padding(
                                        padding: const EdgeInsets.symmetric(vertical: 4),
                                        child: Text(
                                          formattedId.isEmpty ? '--- --- ---' : formattedId,
                                          style: const TextStyle(
                                            fontSize: 28,
                                            fontWeight: FontWeight.w800,
                                            letterSpacing: 1.5,
                                            fontFamily: 'WorkSans',
                                          ),
                                        ),
                                      ),
                                    ),
                                  ],
                                ),
                              ),

                              // 列 2: 验证方式与临时密码
                              Expanded(
                                flex: 5,
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Row(
                                      children: [
                                        Text(
                                          '验证方式: 仅使用临时验证码',
                                          style: TextStyle(
                                            fontSize: 13,
                                            color: isDark ? Colors.grey[400] : const Color(0xFF64748B),
                                          ),
                                        ),
                                      ],
                                    ),
                                    const SizedBox(height: 8),
                                    Row(
                                      children: [
                                        Obx(
                                          () => Text(
                                            _obscurePassword.value
                                                ? '••••••••'
                                                : (myPassword.isEmpty ? '------' : myPassword),
                                            style: TextStyle(
                                              fontSize: 22,
                                              fontWeight: FontWeight.bold,
                                              letterSpacing: _obscurePassword.value ? 4.0 : 1.2,
                                            ),
                                          ),
                                        ),
                                        const SizedBox(width: 14),
                                        // 密码眼睛明暗切换
                                        Obx(
                                          () => IconButton(
                                            icon: Icon(
                                              _obscurePassword.value
                                                  ? Icons.visibility_off_outlined
                                                  : Icons.visibility_outlined,
                                              size: 20,
                                              color: Colors.grey,
                                            ),
                                            tooltip: _obscurePassword.value ? '查看密码' : '隐藏密码',
                                            onPressed: () => _obscurePassword.toggle(),
                                          ),
                                        ),
                                        // 刷新密码图标
                                        AnimatedRotationWidget(
                                          onPressed: () {
                                            bind.mainUpdateTemporaryPassword();
                                            showToast('已刷新临时验证码');
                                          },
                                          child: const Tooltip(
                                            message: '刷新验证码',
                                            child: Icon(
                                              Icons.refresh,
                                              size: 20,
                                              color: Colors.grey,
                                            ),
                                          ),
                                        ),
                                      ],
                                    ),
                                    Text(
                                      '每次远控结束后可自动或手动刷新',
                                      style: TextStyle(
                                        fontSize: 11,
                                        color: isDark ? Colors.grey[500] : const Color(0xFF94A3B8),
                                      ),
                                    ),
                                  ],
                                ),
                              ),

                              // 列 3: 复制并分享按钮
                              Expanded(
                                flex: 3,
                                child: Align(
                                  alignment: Alignment.centerRight,
                                  child: OutlinedButton.icon(
                                    style: OutlinedButton.styleFrom(
                                      padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 14),
                                      side: BorderSide(color: primaryColor, width: 1.2),
                                      shape: RoundedRectangleBorder(
                                        borderRadius: BorderRadius.circular(10),
                                      ),
                                    ),
                                    onPressed: () => _copyAndShare(myId, myPassword),
                                    icon: Icon(Icons.share_outlined, size: 18, color: primaryColor),
                                    label: Text(
                                      '复制并分享',
                                      style: TextStyle(
                                        fontSize: 14,
                                        fontWeight: FontWeight.w600,
                                        color: primaryColor,
                                      ),
                                    ),
                                  ),
                                ),
                              ),
                            ],
                          );
                        },
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 24),

                // 卡片 2: 远控伙伴设备主控面板
                Container(
                  width: double.infinity,
                  padding: const EdgeInsets.all(24),
                  decoration: BoxDecoration(
                    color: cardBg,
                    borderRadius: BorderRadius.circular(16),
                    border: Border.all(color: borderColor, width: 1),
                    boxShadow: [
                      BoxShadow(
                        color: Colors.black.withOpacity(isDark ? 0.2 : 0.04),
                        blurRadius: 12,
                        offset: const Offset(0, 4),
                      ),
                    ],
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const Text(
                        '远控伙伴设备',
                        style: TextStyle(
                          fontSize: 18,
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                      const SizedBox(height: 6),
                      Text(
                        '通过输入对方设备的【设备ID】即可发起远程协助连接',
                        style: TextStyle(
                          fontSize: 13,
                          color: isDark ? Colors.grey[400] : const Color(0xFF64748B),
                        ),
                      ),
                      const Divider(height: 28),

                      Text(
                        '伙伴的设备ID',
                        style: TextStyle(
                          fontSize: 13,
                          fontWeight: FontWeight.w600,
                          color: isDark ? Colors.grey[300] : const Color(0xFF334155),
                        ),
                      ),
                      const SizedBox(height: 10),

                      // 输入框与连接按钮行
                      Row(
                        children: [
                          Expanded(
                            child: Container(
                              height: 48,
                              decoration: BoxDecoration(
                                color: isDark ? const Color(0xFF0F172A) : const Color(0xFFF8FAFC),
                                borderRadius: BorderRadius.circular(10),
                                border: Border.all(color: borderColor),
                              ),
                              child: TextField(
                                controller: _partnerIdController,
                                inputFormatters: [IDTextInputFormatter()],
                                keyboardType: TextInputType.number,
                                style: const TextStyle(
                                  fontSize: 18,
                                  fontWeight: FontWeight.w600,
                                  fontFamily: 'WorkSans',
                                  letterSpacing: 1.0,
                                ),
                                decoration: const InputDecoration(
                                  hintText: '请输入伙伴设备ID (如 964 887 046)',
                                  hintStyle: TextStyle(
                                    fontSize: 14,
                                    fontWeight: FontWeight.normal,
                                    color: Colors.grey,
                                  ),
                                  prefixIcon: Icon(Icons.computer_outlined, color: Colors.grey),
                                  border: InputBorder.none,
                                  contentPadding: EdgeInsets.symmetric(horizontal: 14, vertical: 12),
                                ),
                                onSubmitted: (_) => _onConnectPartner(),
                              ),
                            ),
                          ),
                          const SizedBox(width: 16),

                          // 连接按钮
                          SizedBox(
                            height: 48,
                            width: 130,
                            child: ElevatedButton(
                              style: ElevatedButton.styleFrom(
                                backgroundColor: primaryColor,
                                foregroundColor: Colors.white,
                                elevation: 0,
                                shape: RoundedRectangleBorder(
                                  borderRadius: BorderRadius.circular(10),
                                ),
                              ),
                              onPressed: _onConnectPartner,
                              child: const Text(
                                '连  接',
                                style: TextStyle(
                                  fontSize: 16,
                                  fontWeight: FontWeight.bold,
                                  letterSpacing: 1.5,
                                ),
                              ),
                            ),
                          ),
                        ],
                      ),
                    ],
                  ),
                ),
              ],
            ),
          );
        },
      ),
    );
  }
}
