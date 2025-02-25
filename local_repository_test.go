package main

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"sync"
	"testing"

	"github.com/Songmu/gitconfig"
)

func samePathSlice(lhss, rhss []string) bool {
	sort.Strings(lhss)
	sort.Strings(rhss)
	for i := range lhss {
		if !samePath(lhss[i], rhss[i]) {
			return false
		}
	}
	return true
}

func TestLocalRepositoryFromFullPath(t *testing.T) {
	defer func(orig []string) { _localRepositoryRoots = orig }(_localRepositoryRoots)
	tmproot := newTempDir(t)
	_localRepositoryRoots = []string{tmproot}

	testCases := []struct {
		fpath    string
		expect   string
		subpaths []string
	}{{
		fpath:    filepath.Join(tmproot, "github.com/motemen/ghq"),
		expect:   "motemen/ghq",
		subpaths: []string{"ghq", "motemen/ghq", "github.com/motemen/ghq"},
	}, {
		fpath:    filepath.Join(tmproot, "stash.com/scm/motemen/ghq"),
		expect:   "scm/motemen/ghq",
		subpaths: []string{"ghq", "motemen/ghq", "scm/motemen/ghq", "stash.com/scm/motemen/ghq"},
	}}

	for _, tc := range testCases {
		t.Run(tc.fpath, func(t *testing.T) {
			r, err := LocalRepositoryFromFullPath(tc.fpath, nil)
			if err != nil {
				t.Errorf("error should be nil but: %s", err)
				return
			}
			if r.NonHostPath() != tc.expect {
				t.Errorf("NonHostPath: got: %s, expect: %s", r.NonHostPath(), tc.expect)
			}
			if !reflect.DeepEqual(r.Subpaths(), tc.subpaths) {
				t.Errorf("Subpaths:\ngot:    %+v\nexpect: %+v", r.Subpaths(), tc.subpaths)
			}
		})
	}
}

func TestNewLocalRepository(t *testing.T) {
	defer func(orig []string) { _localRepositoryRoots = orig }(_localRepositoryRoots)
	tmproot := newTempDir(t)
	_localRepositoryRoots = []string{tmproot}

	testCases := []struct {
		name, url, expect string
	}{{
		name:   "GitHub",
		url:    "ssh://git@github.com/motemen/ghq.git",
		expect: filepath.Join(tmproot, "github.com/motemen/ghq"),
	}, {
		name:   "stash",
		url:    "ssh://git@stash.com/scm/motemen/ghq.git",
		expect: filepath.Join(tmproot, "stash.com/scm/motemen/ghq"),
	}, {
		name:   "svn Sourceforge",
		url:    "http://svn.code.sf.net/p/ghq/code/trunk",
		expect: filepath.Join(tmproot, "svn.code.sf.net/p/ghq/code/trunk"),
	}, {
		name:   "git Sourceforge",
		url:    "http://git.code.sf.net/p/ghq/code",
		expect: filepath.Join(tmproot, "git.code.sf.net/p/ghq/code"),
	}, {
		name:   "svn Sourceforge JP",
		url:    "http://scm.sourceforge.jp/svnroot/ghq/",
		expect: filepath.Join(tmproot, "scm.sourceforge.jp/svnroot/ghq"),
	}, {
		name:   "git Sourceforge JP",
		url:    "http://scm.sourceforge.jp/gitroot/ghq/ghq.git",
		expect: filepath.Join(tmproot, "scm.sourceforge.jp/gitroot/ghq/ghq"),
	}, {
		name:   "svn Assembla",
		url:    "https://subversion.assembla.com/svn/ghq/",
		expect: filepath.Join(tmproot, "subversion.assembla.com/svn/ghq"),
	}, {
		name:   "git Assembla",
		url:    "https://git.assembla.com/ghq.git",
		expect: filepath.Join(tmproot, "git.assembla.com/ghq"),
	}, {
		name:   "bitbucket host with port",
		url:    "https://bitbucket.local:8888/motemen/ghq.git",
		expect: filepath.Join(tmproot, "bitbucket.local/motemen/ghq"),
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func(orig string) { _home = orig }(_home)
			_home = ""
			homeOnce = &sync.Once{}
			r, err := LocalRepositoryFromURL(mustParseURL(tc.url))
			if err != nil {
				t.Errorf("error should be nil but: %s", err)
			}
			if r.FullPath != tc.expect {
				t.Errorf("got: %s, expect: %s", r.FullPath, tc.expect)
			}
		})
	}
}

func TestLocalRepositoryRoots(t *testing.T) {
	defer func(orig []string) { _localRepositoryRoots = orig }(_localRepositoryRoots)

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		root   string
		expect []string
	}{{
		root:   "/path/to/ghqroot",
		expect: []string{"/path/to/ghqroot"},
	}, {
		root:   "/path/to/ghqroot1" + string(os.PathListSeparator) + "/path/to/ghqroot2",
		expect: []string{"/path/to/ghqroot1", "/path/to/ghqroot2"},
	}, {
		root:   "/path/to/ghqroot11" + string(os.PathListSeparator) + "vendor",
		expect: []string{"/path/to/ghqroot11", filepath.Join(wd, "vendor")},
	}}

	for _, tc := range testCases {
		t.Run(tc.root, func(t *testing.T) {
			_localRepositoryRoots = nil
			localRepoOnce = &sync.Once{}
			setEnv(t, envGhqRoot, tc.root)
			got, err := localRepositoryRoots(true)
			if err != nil {
				t.Errorf("error should be nil, but: %s", err)
			}
			if !samePathSlice(got, tc.expect) {
				t.Errorf("\ngot:    %+v\nexpect: %+v", got, tc.expect)
			}
		})
	}
}

// https://gist.github.com/kyanny/c231f48e5d08b98ff2c3
func TestList_Symlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.SkipNow()
	}
	root := newTempDir(t)
	symDir := newTempDir(t)

	origLocalRepositryRoots := _localRepositoryRoots
	_localRepositoryRoots = []string{root}
	defer func() { _localRepositoryRoots = origLocalRepositryRoots }()

	if err := os.MkdirAll(filepath.Join(root, "github.com", "atom", "atom", ".git"), 0777); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(root, "github.com", "zabbix", "zabbix", ".git"), 0777); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(symDir, filepath.Join(root, "github.com", "ghq")); err != nil {
		t.Fatal(err)
	}

	var paths []string
	walkAllLocalRepositories(func(repo *LocalRepository) {
		paths = append(paths, repo.RelPath)
	})

	if len(paths) != 2 {
		t.Errorf("length of paths should be 2, but: %d", len(paths))
	}
}

func TestList_Symlink_In_Same_Directory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.SkipNow()
	}
	root := newTempDir(t)
	symDir := newTempDir(t)

	origLocalRepositryRoots := _localRepositoryRoots
	_localRepositoryRoots = []string{root}
	defer func() { _localRepositoryRoots = origLocalRepositryRoots }()

	if err := os.MkdirAll(filepath.Join(root, "github.com", "root-user", "a-repository", ".git"), 0777); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(root, "github.com", "root-user", "z-repository", ".git"), 0777); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(symDir, "github.com", "sym-user", "h-repository", ".git"), 0777); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(filepath.Join(symDir, "github.com", "sym-user", "h-repository"), filepath.Join(root, "github.com", "root-user", "h-repository")); err != nil {
		t.Fatal(err)
	}

	var paths []string
	walkAllLocalRepositories(func(repo *LocalRepository) {
		paths = append(paths, repo.RelPath)
	})

	if len(paths) != 3 {
		t.Errorf("length of paths should be 3, but: %d", len(paths))
	}
}

func TestFindVCSBackend(t *testing.T) {
	testCases := []struct {
		name   string
		setup  func(t *testing.T) (string, string)
		expect *VCSBackend
	}{{
		name: "git",
		setup: func(t *testing.T) (string, string) {
			dir := newTempDir(t)
			os.MkdirAll(filepath.Join(dir, ".git"), 0755)
			return dir, ""
		},
		expect: GitBackend,
	}, {
		name: "git svn",
		setup: func(t *testing.T) (string, string) {
			dir := newTempDir(t)
			os.MkdirAll(filepath.Join(dir, ".git", "svn"), 0755)
			return dir, ""
		},
		expect: GitBackend,
	}, {
		name: "git with matched vcs",
		setup: func(t *testing.T) (string, string) {
			dir := newTempDir(t)
			os.MkdirAll(filepath.Join(dir, ".git"), 0755)
			return dir, "git"
		},
		expect: GitBackend,
	}, {
		name: "git with not matched vcs",
		setup: func(t *testing.T) (string, string) {
			dir := newTempDir(t)
			os.MkdirAll(filepath.Join(dir, ".git"), 0755)
			return dir, "mercurial"
		},
		expect: nil,
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fpath, vcs := tc.setup(t)
			backend := findVCSBackend(fpath, vcs)
			if backend != tc.expect {
				t.Errorf("got: %v, expect: %v", backend, tc.expect)
			}
		})
	}
}

func TestLocalRepository_VCS(t *testing.T) {
	defer func(orig []string) { _localRepositoryRoots = orig }(_localRepositoryRoots)

	_localRepositoryRoots = nil
	localRepoOnce = &sync.Once{}
	tmpdir := newTempDir(t)
	setEnv(t, envGhqRoot, tmpdir)

	pkg := filepath.Join(tmpdir, "github.com", "motemen", "ghq")
	subpkg := filepath.Join(pkg, "logger")

	os.MkdirAll(filepath.Join(pkg, ".git"), 0755)
	os.MkdirAll(subpkg, 0755)

	t.Run("reporoot", func(t *testing.T) {
		repo, err := LocalRepositoryFromFullPath(pkg, nil)
		if err != nil {
			t.Errorf("error should be nil, but: %s (%s)", err, pkg)
			return
		}
		vcs, repoPath := repo.VCS()
		if vcs != GitBackend {
			t.Errorf("repo.VCS() = %+v, expect: GitBackend", vcs)
			return
		}
		if repoPath != pkg {
			t.Errorf("got: %s, expect: %s", repoPath, pkg)
		}
	})

	t.Run("subdir", func(t *testing.T) {
		repo, err := LocalRepositoryFromFullPath(subpkg, nil)
		if err != nil {
			t.Errorf("error should be nil, but: %s", err)
			return
		}
		vcs, repoPath := repo.VCS()
		if vcs != GitBackend {
			t.Errorf("repo.VCS() = %+v, expect: GitBackend", vcs)
		}
		if repoPath != pkg {
			t.Errorf("got: %s, expect: %s", repoPath, pkg)
		}
	})
}

func TestLocalRepositoryRoots_URLMatchLocalRepositoryRoots(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.SkipNow()
	}
	setEnv(t, "HOME", "/home/tmp")
	defer func(orig string) { _home = orig }(_home)

	_home = ""
	homeOnce = &sync.Once{}
	t.Cleanup(gitconfig.WithConfig(t, `
[ghq]
  root = /hoge
[ghq "https://github.com/hatena"]
  root = ~/proj/hatena
  root = /backups/hatena
[ghq "https://github.com/natureglobal"]
  root = ~/proj/natureglobal
`))

	want := []string{"/hoge", "/home/tmp/proj/hatena", "/backups/hatena", "/home/tmp/proj/natureglobal"}

	_localRepositoryRoots = nil
	localRepoOnce = &sync.Once{}
	got, err := localRepositoryRoots(true)
	if err != nil {
		t.Errorf("error should be nil but: %s", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("localRepositoryRoots(true) = %+v, want: %+v", got, want)
	}
}
