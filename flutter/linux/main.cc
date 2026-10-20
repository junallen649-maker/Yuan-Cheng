#include <dlfcn.h>
#include <limits.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>
#include "my_application.h"

#define POLARIS_LIB_PATH "libpolaris.so"
#define RUSTDESK_LIB_PATH "librustdesk.so"
typedef bool (*CoreMainFunc)();
bool gIsConnectionManager = false;

void print_help_install_pkg(const char* so);

static void* dlopen_bundled_lib() {
  char exe_path[PATH_MAX];
  ssize_t len = readlink("/proc/self/exe", exe_path, sizeof(exe_path) - 1);
  if (len <= 0 || len >= (ssize_t)(sizeof(exe_path) - 1)) return nullptr;
  exe_path[len] = '\0';
  char* last_slash = strrchr(exe_path, '/');
  if (last_slash == nullptr) return nullptr;
  *last_slash = '\0';

  const char* lib_names[] = {POLARIS_LIB_PATH, RUSTDESK_LIB_PATH};
  for (const char* name : lib_names) {
    char lib_path[PATH_MAX + 64];
    snprintf(lib_path, sizeof(lib_path), "%s/lib/%s", exe_path, name);
    if (access(lib_path, F_OK) == 0) {
      void* lib = dlopen(lib_path, RTLD_LAZY);
      if (lib) return lib;
    }
  }
  return nullptr;
}

bool flutter_polaris_core_main() {
   void* lib = dlopen_bundled_lib();
   if (!lib) {
      lib = dlopen(POLARIS_LIB_PATH, RTLD_LAZY);
   }
   if (!lib) {
      lib = dlopen(RUSTDESK_LIB_PATH, RTLD_LAZY);
   }
   if (!lib) {
      fprintf(stderr,"Failed to load \"%s\" or \"%s\"\n", POLARIS_LIB_PATH, RUSTDESK_LIB_PATH);
      char* error;
      if ((error = dlerror()) != nullptr) {
        fprintf(stderr, "%s\n", error);
        char* libmissed = strstr(error, ": cannot open shared object file: No such file or directory");
        if (libmissed != nullptr) {
          *libmissed = '\0';
          char* so = strdup(error);
          print_help_install_pkg(so);
          free(so);
        }
      }
     return false;
   }
   auto core_main = (CoreMainFunc) dlsym(lib, "polaris_core_main");
   if (!core_main) {
       core_main = (CoreMainFunc) dlsym(lib, "rustdesk_core_main");
   }
   if (!core_main) {
       char* error = dlerror();
       fprintf(stderr, "Program entry \"polaris_core_main\" is not found: %s\n", error ? error : "unknown");
       return false;
   }
   return core_main();
}

int main(int argc, char** argv) {
  if (!flutter_polaris_core_main()) {
      return 0;
  }
  for (int i = 0; i < argc; i++) {
    if (strcmp(argv[i], "--cm") == 0) {
      gIsConnectionManager = true;
    }
  }
  g_autoptr(MyApplication) app = my_application_new();
  return g_application_run(G_APPLICATION(app), argc, argv);
}

typedef struct {
  const char* mgr;
  const char* search;
} PkgMgrSearch;

const PkgMgrSearch g_mgrs[] = {
  {
    "apt",
    "apt-file search",
  },
  {
    "dnf",
    "dnf provides",
  },
  {
    "yum",
    "yum provides",
  },
  {
    "zypper",
    "zypper wp",
  },
  {
    "pacman",
    "pacman -Qo",
  },
  {
    NULL,
    NULL,
  },
};

int is_command_exists(const char* command) {
    char* path = getenv("PATH");
    char* path_copy = strdup(path);
    char* dir = strtok(path_copy, ":");

    while (dir != NULL) {
        char command_path[256];
        snprintf(command_path, sizeof(command_path), "%s/%s", dir, command);
        if (access(command_path, X_OK) == 0) {
            free(path_copy);
            return 1;
        }
        dir = strtok(NULL, ":");
    }

    free(path_copy);
    return 0;
}

// We do not automatically search pkg 
// as the search process can be time consuming and update may be required.
void print_help_install_pkg(const char* so)
{
  if (strcmp(so, "libnsl.so.1") == 0) {
    const char* mgr[] = {"yum", "dnf", NULL};
    const char** m = mgr;
    while (*m != NULL) {
      if (is_command_exists(*m)) {
        fprintf(stderr, "Please run \"%s install libnsl\" to install the required package.\n", *m);
        return;
      }
      m++;
    }
  }

  const PkgMgrSearch *mgr_search = g_mgrs;
  while (mgr_search->mgr != NULL) {
      if (is_command_exists(mgr_search->mgr) == 1) {
        fprintf(stderr, "Please run \"%s %s\" to search and install the pkg.\n", mgr_search->search, so);
        break;
      }
      mgr_search++;
  }
}
