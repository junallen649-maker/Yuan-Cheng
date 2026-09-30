import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_hbb/common.dart';
import 'package:flutter_hbb/common/formatter/id_formatter.dart';
import 'package:flutter_hbb/common/widgets/animated_rotation_widget.dart';
import 'package:flutter_hbb/models/platform_model.dart';
import 'package:flutter_hbb/models/server_model.dart';
import 'package:get/get.dart';
import 'package:provider/provider.dart';

/// UU 远程风格 - 视图 C：远程协助主控与受控面板（带丝滑微动画与交互反馈）
class DeskRemoteAssistView extends StatefulWidget {
  const DeskRemoteAssistView({Key? key}) : super(key: key);

  @override
  State<DeskRemoteAssistView> createState() => _DeskRemoteAssistViewState();
}

class _DeskRemoteAssistViewState extends State<DeskRemoteAssistView> {
  final TextEditingController _partnerIdController = TextEditingController();
  final RxBool _obscurePassword = true.obs;
  final RxBool _allowRemoteControl = true.obs;
  final RxBool _isCopied = false.obs;
  Timer? _copyResetTimer;

  @override
  void initState() {
    super.initState();
    _initServiceState();
  }

  void _initServiceState() {
    // 读取本地配置，无多余提权与命令提示
    final allow = bind.mainGetLocalOption(key: 'allow-remote-control');
    _allowRemoteControl.value = allow != 'N';
  }

  void _toggleAllowRemote(bool val) async {
    _allowRemoteControl.value = val;
    // 纯本地配置切换，绝不调用 Windows 服务停止或卸载，零弹窗零命令提示
    await bind.mainSetLocalOption(key: 'allow-remote-control', value: val ? 'Y' : 'N');
    if (val) {
      await bind.mainSetLocalOption(key: 'access-mode', value: 'full');
      showToast('已开启远程协助');
    } else {
      await bind.mainSetLocalOption(key: 'access-mode', value: 'view');
      showToast('已暂停远程协助');
    }
  }

  void _copyAndShare(String id, String password) {
    final cleanId = formatID(id);
    final text = '【Polaris远程】我的设备ID：$cleanId，临时验证码：$password';
    Clipboard.setData(ClipboardData(text: text));
    _isCopied.value = true;
    _copyResetTimer?.cancel();
    _copyResetTimer = Timer(const Duration(seconds: 2), () {
      if (mounted) {
        _isCopied.value = false;
      }
    });
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
    _copyResetTimer?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final cardBg = isDark ? const Color(0xFF1E293B) : Colors.white;
    final borderColor = isDark ? const Color(0xFF334155) : const Color(0xFFE2E8F0);
    final primaryColor = const Color(0xFF1E6FFF);
    final isOutgoingOnly = bind.isOutgoingOnly();

    return ChangeNotifierProvider.value(
      value: gFFI.serverModel,
      child: Consumer<ServerModel>(
        builder: (context, serverModel, child) {
          final myId = serverModel.serverId.text;
          final myPassword = serverModel.serverPasswd.text;
          final formattedId = formatID(myId);

          return TweenAnimationBuilder<double>(
            tween: Tween<double>(begin: 0.0, end: 1.0),
            duration: const Duration(milliseconds: 280),
            curve: Curves.easeOutCubic,
            builder: (context, animVal, child) {
              return Opacity(
                opacity: animVal,
                child: Transform.translate(
                  offset: Offset(0, 16 * (1.0 - animVal)),
                  child: child,
                ),
              );
            },
            child: SingleChildScrollView(
              padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 28),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // 页面大标题
                  Row(
                    children: [
                      Text(
                        isOutgoingOnly ? '远程控制中心' : '远程协助',
                        style: const TextStyle(
                          fontSize: 24,
                          fontWeight: FontWeight.bold,
                          letterSpacing: 0.5,
                        ),
                      ),
                      const SizedBox(width: 12),
                      Container(
                        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                        decoration: BoxDecoration(
                          color: primaryColor.withOpacity(0.12),
                          borderRadius: BorderRadius.circular(6),
                        ),
                        child: Text(
                          isOutgoingOnly ? '管理控制端' : '专线极速直连',
                          style: TextStyle(
                            fontSize: 11,
                            fontWeight: FontWeight.w600,
                            color: primaryColor,
                          ),
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 20),

                  // 卡片 1: 控制端概览看板 或 受控端卡片
                  if (isOutgoingOnly)
                    _buildConsoleAssetDashboard(
                      context: context,
                      isDark: isDark,
                      cardBg: cardBg,
                      borderColor: borderColor,
                      primaryColor: primaryColor,
                    )
                  else
                    _HoverElevationCard(
                      isDark: isDark,
                      cardBg: cardBg,
                      borderColor: borderColor,
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Row(
                            mainAxisAlignment: MainAxisAlignment.spaceBetween,
                            children: [
                              Row(
                                children: [
                                  Container(
                                    width: 8,
                                    height: 8,
                                    decoration: const BoxDecoration(
                                      color: Color(0xFF10B981),
                                      shape: BoxShape.circle,
                                    ),
                                  ),
                                  const SizedBox(width: 8),
                                  const Text(
                                    '本设备受控端',
                                    style: TextStyle(
                                      fontSize: 18,
                                      fontWeight: FontWeight.bold,
                                    ),
                                  ),
                                ],
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
                          LayoutBuilder(
                            builder: (context, constraints) {
                              return Row(
                                crossAxisAlignment: CrossAxisAlignment.center,
                                children: [
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
                                        _HoverClickCopyWidget(
                                          text: myId,
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
                                      ],
                                    ),
                                  ),
                                  Expanded(
                                    flex: 5,
                                    child: Column(
                                      crossAxisAlignment: CrossAxisAlignment.start,
                                      children: [
                                        Text(
                                          '验证方式: 仅使用临时验证码',
                                          style: TextStyle(
                                            fontSize: 13,
                                            color: isDark ? Colors.grey[400] : const Color(0xFF64748B),
                                          ),
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
                                  Expanded(
                                    flex: 3,
                                    child: Align(
                                      alignment: Alignment.centerRight,
                                      child: Obx(() {
                                        final copied = _isCopied.value;
                                        return AnimatedContainer(
                                          duration: const Duration(milliseconds: 220),
                                          curve: Curves.easeOutCubic,
                                          decoration: BoxDecoration(
                                            color: copied
                                                ? const Color(0xFF10B981).withOpacity(0.12)
                                                : primaryColor.withOpacity(0.06),
                                            borderRadius: BorderRadius.circular(10),
                                            border: Border.all(
                                              color: copied ? const Color(0xFF10B981) : primaryColor,
                                              width: 1.2,
                                            ),
                                          ),
                                          child: Material(
                                            color: Colors.transparent,
                                            child: InkWell(
                                              borderRadius: BorderRadius.circular(10),
                                              onTap: () => _copyAndShare(myId, myPassword),
                                              child: Padding(
                                                padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
                                                child: Row(
                                                  mainAxisSize: MainAxisSize.min,
                                                  children: [
                                                    AnimatedSwitcher(
                                                      duration: const Duration(milliseconds: 200),
                                                      child: Icon(
                                                        copied
                                                            ? Icons.check_circle_rounded
                                                            : Icons.share_outlined,
                                                        key: ValueKey(copied),
                                                        size: 18,
                                                        color: copied
                                                            ? const Color(0xFF10B981)
                                                            : primaryColor,
                                                      ),
                                                    ),
                                                    const SizedBox(width: 8),
                                                    Text(
                                                      copied ? '已复制分享' : '复制并分享',
                                                      style: TextStyle(
                                                        fontSize: 13,
                                                        fontWeight: FontWeight.w600,
                                                        color: copied
                                                            ? const Color(0xFF10B981)
                                                            : primaryColor,
                                                      ),
                                                    ),
                                                  ],
                                                ),
                                              ),
                                            ),
                                          ),
                                        );
                                      }),
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

                  // 卡片 2: 远控受控设备主控面板
                  _HoverElevationCard(
                    isDark: isDark,
                    cardBg: cardBg,
                    borderColor: borderColor,
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          isOutgoingOnly ? '发起远程控制' : '远控伙伴设备',
                          style: const TextStyle(
                            fontSize: 18,
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                        const SizedBox(height: 6),
                        Text(
                          isOutgoingOnly
                              ? '输入已授权的受控端设备ID即可快速建立安全远程桌面'
                              : '通过输入对方设备的【设备ID】即可发起远程协助连接',
                          style: TextStyle(
                            fontSize: 13,
                            color: isDark ? Colors.grey[400] : const Color(0xFF64748B),
                          ),
                        ),
                        const Divider(height: 28),

                        Text(
                          isOutgoingOnly ? '受控设备 ID' : '伙伴的设备ID',
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
                                  decoration: InputDecoration(
                                    hintText: isOutgoingOnly
                                        ? '请输入受控端设备ID (如 964 887 046)'
                                        : '请输入伙伴设备ID (如 964 887 046)',
                                    hintStyle: const TextStyle(
                                      fontSize: 14,
                                      fontWeight: FontWeight.normal,
                                      color: Colors.grey,
                                    ),
                                    prefixIcon: const Icon(Icons.computer_outlined, color: Colors.grey),
                                    border: InputBorder.none,
                                    contentPadding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
                                  ),
                                  onSubmitted: (_) => _onConnectPartner(),
                                ),
                              ),
                            ),
                            const SizedBox(width: 16),

                            // 丝滑微动效连接按钮
                            _InteractiveConnectButton(
                              onPressed: _onConnectPartner,
                            ),
                          ],
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          );
        },
      ),
    );
  }

  Widget _buildConsoleAssetDashboard({
    required BuildContext context,
    required bool isDark,
    required Color cardBg,
    required Color borderColor,
    required Color primaryColor,
  }) {
    return AnimatedBuilder(
      animation: gFFI.recentPeersModel,
      builder: (context, _) {
        final peers = gFFI.recentPeersModel.peers;
        final totalCount = peers.length;
        final onlineCount = peers.where((p) => p.online).length;

        return _HoverElevationCard(
          isDark: isDark,
          cardBg: cardBg,
          borderColor: borderColor,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Row(
                    children: [
                      Container(
                        width: 8,
                        height: 8,
                        decoration: const BoxDecoration(
                          color: Color(0xFF10B981),
                          shape: BoxShape.circle,
                        ),
                      ),
                      const SizedBox(width: 8),
                      const Text(
                        '受控设备概况',
                        style: TextStyle(
                          fontSize: 18,
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                    ],
                  ),
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                    decoration: BoxDecoration(
                      color: const Color(0xFF10B981).withOpacity(0.12),
                      borderRadius: BorderRadius.circular(6),
                    ),
                    child: const Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(Icons.shield_outlined, size: 14, color: Color(0xFF10B981)),
                        SizedBox(width: 4),
                        Text(
                          '专业控制端 · 安全就绪',
                          style: TextStyle(
                            fontSize: 12,
                            fontWeight: FontWeight.w600,
                            color: Color(0xFF10B981),
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
              const Divider(height: 28),
              Row(
                children: [
                  Expanded(
                    child: _buildMetricTile(
                      isDark: isDark,
                      title: '已授权受控设备',
                      value: '$totalCount 台',
                      icon: Icons.devices_rounded,
                      color: primaryColor,
                    ),
                  ),
                  const SizedBox(width: 14),
                  Expanded(
                    child: _buildMetricTile(
                      isDark: isDark,
                      title: '实时在线被控端',
                      value: '$onlineCount 台',
                      icon: Icons.wifi_tethering_rounded,
                      color: const Color(0xFF10B981),
                    ),
                  ),
                  const SizedBox(width: 14),
                  Expanded(
                    child: _buildMetricTile(
                      isDark: isDark,
                      title: '接入专线节点',
                      value: '8.138.129.79',
                      icon: Icons.hub_rounded,
                      color: const Color(0xFF8B5CF6),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 14),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
                decoration: BoxDecoration(
                  color: isDark ? const Color(0xFF0F172A) : const Color(0xFFF1F5F9),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Row(
                  children: [
                    Icon(Icons.info_outline, size: 16, color: isDark ? Colors.grey[400] : const Color(0xFF64748B)),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        '本客户端当前以独立控制端模式运行，本机受控端口已隔离。远程控制、文件传输与远程开机功能由控制端单向发起。',
                        style: TextStyle(
                          fontSize: 12,
                          color: isDark ? Colors.grey[400] : const Color(0xFF64748B),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
        );
      },
    );
  }

  Widget _buildMetricTile({
    required bool isDark,
    required String title,
    required String value,
    required IconData icon,
    required Color color,
  }) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: isDark ? const Color(0xFF0F172A) : const Color(0xFFF8FAFC),
        borderRadius: BorderRadius.circular(10),
        border: Border.all(
          color: isDark ? const Color(0xFF334155) : const Color(0xFFE2E8F0),
        ),
      ),
      child: Row(
        children: [
          Container(
            padding: const EdgeInsets.all(8),
            decoration: BoxDecoration(
              color: color.withOpacity(0.12),
              borderRadius: BorderRadius.circular(8),
            ),
            child: Icon(icon, color: color, size: 20),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: TextStyle(
                    fontSize: 11,
                    color: isDark ? Colors.grey[400] : const Color(0xFF64748B),
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  value,
                  style: const TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.bold,
                  ),
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// 鼠标悬浮微阴影卡片容器
class _HoverElevationCard extends StatefulWidget {
  final bool isDark;
  final Color cardBg;
  final Color borderColor;
  final Widget child;

  const _HoverElevationCard({
    Key? key,
    required this.isDark,
    required this.cardBg,
    required this.borderColor,
    required this.child,
  }) : super(key: key);

  @override
  State<_HoverElevationCard> createState() => _HoverElevationCardState();
}

class _HoverElevationCardState extends State<_HoverElevationCard> {
  bool _isHovered = false;

  @override
  Widget build(BuildContext context) {
    return MouseRegion(
      onEnter: (_) => setState(() => _isHovered = true),
      onExit: (_) => setState(() => _isHovered = false),
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 200),
        curve: Curves.easeOutCubic,
        width: double.infinity,
        padding: const EdgeInsets.all(24),
        decoration: BoxDecoration(
          color: widget.cardBg,
          borderRadius: BorderRadius.circular(16),
          border: Border.all(
            color: _isHovered ? const Color(0xFF1E6FFF).withOpacity(0.4) : widget.borderColor,
            width: _isHovered ? 1.2 : 1,
          ),
          boxShadow: [
            BoxShadow(
              color: Colors.black.withOpacity(widget.isDark ? 0.25 : (_isHovered ? 0.08 : 0.04)),
              blurRadius: _isHovered ? 18 : 12,
              offset: Offset(0, _isHovered ? 6 : 4),
            ),
          ],
        ),
        child: widget.child,
      ),
    );
  }
}

/// 悬浮高亮点击复制组件
class _HoverClickCopyWidget extends StatefulWidget {
  final String text;
  final Widget child;

  const _HoverClickCopyWidget({Key? key, required this.text, required this.child}) : super(key: key);

  @override
  State<_HoverClickCopyWidget> createState() => _HoverClickCopyWidgetState();
}

class _HoverClickCopyWidgetState extends State<_HoverClickCopyWidget> {
  bool _isHovered = false;

  @override
  Widget build(BuildContext context) {
    return MouseRegion(
      cursor: SystemMouseCursors.click,
      onEnter: (_) => setState(() => _isHovered = true),
      onExit: (_) => setState(() => _isHovered = false),
      child: GestureDetector(
        onTap: () {
          Clipboard.setData(ClipboardData(text: widget.text));
          showToast('设备ID已复制');
        },
        child: Tooltip(
          message: '点击复制设备ID',
          child: AnimatedContainer(
            duration: const Duration(milliseconds: 150),
            padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
            decoration: BoxDecoration(
              color: _isHovered ? const Color(0xFF1E6FFF).withOpacity(0.08) : Colors.transparent,
              borderRadius: BorderRadius.circular(8),
            ),
            child: widget.child,
          ),
        ),
      ),
    );
  }
}

/// 带有按压缩放与悬浮光晕的连接按钮
class _InteractiveConnectButton extends StatefulWidget {
  final VoidCallback onPressed;

  const _InteractiveConnectButton({Key? key, required this.onPressed}) : super(key: key);

  @override
  State<_InteractiveConnectButton> createState() => _InteractiveConnectButtonState();
}

class _InteractiveConnectButtonState extends State<_InteractiveConnectButton> {
  bool _isHovered = false;
  bool _isPressed = false;

  @override
  Widget build(BuildContext context) {
    return MouseRegion(
      cursor: SystemMouseCursors.click,
      onEnter: (_) => setState(() => _isHovered = true),
      onExit: (_) => setState(() {
        _isHovered = false;
        _isPressed = false;
      }),
      child: GestureDetector(
        onTapDown: (_) => setState(() => _isPressed = true),
        onTapUp: (_) => setState(() => _isPressed = false),
        onTapCancel: () => setState(() => _isPressed = false),
        onTap: widget.onPressed,
        child: AnimatedScale(
          scale: _isPressed ? 0.96 : (_isHovered ? 1.02 : 1.0),
          duration: const Duration(milliseconds: 120),
          curve: Curves.easeOutCubic,
          child: AnimatedContainer(
            duration: const Duration(milliseconds: 200),
            height: 48,
            width: 130,
            decoration: BoxDecoration(
              gradient: LinearGradient(
                colors: _isHovered
                    ? [const Color(0xFF2B7BFF), const Color(0xFF1E6FFF)]
                    : [const Color(0xFF1E6FFF), const Color(0xFF1557CC)],
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
              ),
              borderRadius: BorderRadius.circular(10),
              boxShadow: [
                BoxShadow(
                  color: const Color(0xFF1E6FFF).withOpacity(_isHovered ? 0.45 : 0.2),
                  blurRadius: _isHovered ? 14 : 6,
                  offset: const Offset(0, 3),
                ),
              ],
            ),
            alignment: Alignment.center,
            child: const Text(
              '连  接',
              style: TextStyle(
                fontSize: 16,
                fontWeight: FontWeight.bold,
                letterSpacing: 1.5,
                color: Colors.white,
              ),
            ),
          ),
        ),
      ),
    );
  }
}
