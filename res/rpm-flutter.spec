Name:       polaris
Version:    1.5.0
Release:    0
Summary:    RPM package
License:    GPL-3.0
URL:        https://polaris.com
Vendor:     Polaris <support@polaris.com>
Requires:   gtk3 libxcb libXfixes alsa-lib libva gstreamer1-plugins-base
Recommends: libayatana-appindicator-gtk3 libxdo
Provides:   libdesktop_drop_plugin.so()(64bit), libdesktop_multi_window_plugin.so()(64bit), libfile_selector_linux_plugin.so()(64bit), libflutter_custom_cursor_plugin.so()(64bit), libflutter_linux_gtk.so()(64bit), libscreen_retriever_plugin.so()(64bit), libtray_manager_plugin.so()(64bit), liburl_launcher_linux_plugin.so()(64bit), libwindow_manager_plugin.so()(64bit), libwindow_size_plugin.so()(64bit), libtexture_rgba_renderer_plugin.so()(64bit)

# https://docs.fedoraproject.org/en-US/packaging-guidelines/Scriptlets/

%description
Polaris Remote Desktop software.

%prep
# we have no source, so nothing here

%build
# we have no source, so nothing here

# %global __python %{__python3}

%install

mkdir -p "%{buildroot}/usr/share/polaris" && cp -r ${HBB}/flutter/build/linux/x64/release/bundle/* -t "%{buildroot}/usr/share/polaris"
mkdir -p "%{buildroot}/usr/bin"
install -Dm 644 $HBB/res/polaris.service -t "%{buildroot}/usr/share/polaris/files"
install -Dm 644 $HBB/res/polaris.desktop -t "%{buildroot}/usr/share/polaris/files"
install -Dm 644 $HBB/res/polaris-link.desktop -t "%{buildroot}/usr/share/polaris/files"
install -Dm 644 $HBB/res/128x128@2x.png "%{buildroot}/usr/share/icons/hicolor/256x256/apps/polaris.png"
install -Dm 644 $HBB/res/scalable.svg "%{buildroot}/usr/share/icons/hicolor/scalable/apps/polaris.svg"

%files
/usr/share/polaris/*
/usr/share/polaris/files/polaris.service
/usr/share/icons/hicolor/256x256/apps/polaris.png
/usr/share/icons/hicolor/scalable/apps/polaris.svg
/usr/share/polaris/files/polaris.desktop
/usr/share/polaris/files/polaris-link.desktop

%changelog
# let's skip this for now

%pre
# can do something for centos7
case "$1" in
  1)
    # for install
  ;;
  2)
    # for upgrade
    systemctl stop polaris || true
  ;;
esac

%post
cp /usr/share/polaris/files/polaris.service /etc/systemd/system/polaris.service
cp /usr/share/polaris/files/polaris.desktop /usr/share/applications/
cp /usr/share/polaris/files/polaris-link.desktop /usr/share/applications/
ln -sf /usr/share/polaris/polaris /usr/bin/polaris
systemctl daemon-reload
systemctl enable polaris
systemctl start polaris
update-desktop-database

%preun
case "$1" in
  0)
    # for uninstall
    systemctl stop polaris || true
    systemctl disable polaris || true
    rm /etc/systemd/system/polaris.service || true
  ;;
  1)
    # for upgrade
  ;;
esac

%postun
case "$1" in
  0)
    # for uninstall
    rm /usr/bin/polaris || true
    rmdir /usr/lib/polaris || true
    rmdir /usr/local/polaris || true
    rmdir /usr/share/polaris || true
    rm /usr/share/applications/polaris.desktop || true
    rm /usr/share/applications/polaris-link.desktop || true
    update-desktop-database
  ;;
  1)
    # for upgrade
    rmdir /usr/lib/polaris || true
    rmdir /usr/local/polaris || true
  ;;
esac
