import importlib.util, json, os, sqlite3, subprocess, sys, tempfile
from pathlib import Path
root = Path.cwd()
evidence = Path('/Users/carpenter/.no-mistakes/evidence/01M3ZVZARP4ZQS3VNJAMWWA6VN')
spec = importlib.util.spec_from_file_location('smoke', root / 'scripts/smoke.py')
s = importlib.util.module_from_spec(spec)
spec.loader.exec_module(s)
tempfile.tempdir = str(root / '.test-local')
original_close = s.Terminal.close
count = 0
def close(self):
    global count
    count += 1
    (evidence / f'live-terminal-{count}.ansi').write_bytes(self.output)
    original_close(self)
s.Terminal.close = close
os.environ['HISTFILE'] = '/dev/null'
s.main()
s.PRACTICE_ENV.clear()
cat = json.loads((root / 'internal/catalog/data/catalog.json').read_text())
print(type(cat), flush=True)
with tempfile.TemporaryDirectory(prefix='revision-live-') as directory:
    state = Path(directory)
    for exercise in ('git.unstage-preserve', 'git.restore-one'):
        s.session(state, ['play', exercise], b'exit\n', 1)
        attempts = json.loads(s.plain(state, 'export'))['attempts']
        current = next(a for a in attempts if a['exercise_id'] == exercise and a['revision'] == 2)
        assert current['status'] == 'active', current
        # Seed the persisted pre-update attempt contract, with its own workspace.
        old = dict(current, id=current['id'] + '-old', revision=1, workspace=current['workspace'] + '-old')
        with sqlite3.connect(state / 'driving-range.db') as db:
            db.execute('INSERT INTO attempts(id,data) VALUES(?,?)', (old['id'], json.dumps(old)))
        s.session(state, ['play', old['id']], b'exit\n', 1)
        for a in (current, old):
            definitions = cat['challenges'] if 'challenges' in cat else cat['exercises']
            ch = next(c for c in definitions if c['id'] == exercise and c['revision'] == a['revision'])
            name = 'validation-' + a['id']
            args = ['docker', 'create', '--name', name, '--pull=never', '--network=none', '--user=1000:1000', '--cap-drop=ALL', '--security-opt=no-new-privileges', '--read-only', '--cpus=1', '--memory=512m', '--memory-swap=512m', '--pids-limit=128', '--ulimit=nofile=256:256', '--ulimit=fsize=16777216:16777216', '--tmpfs=/tmp:rw,nosuid,nodev,size=64m', '--mount', 'type=volume,source='+a['workspace']+',target=/workspace', '--workdir=/workspace', os.environ['GOLF_IMAGE'], 'bash', '-c', ch['reference_solution']]
            subprocess.run(args, check=True, capture_output=True, timeout=30)
            try:
                subprocess.run(['docker', 'start', '-a', name], check=True, timeout=30)
                subprocess.run(['docker', 'cp', name+':/workspace/.', str(state/'workspaces'/a['workspace']/'files')], check=True, timeout=30)
            finally:
                subprocess.run(['docker', 'rm', '-f', name], check=True, capture_output=True, timeout=30)
            s.session(state, ['play', a['id']], b'exit\n', 0)
            result = json.loads(s.plain(state, 'export'))
            saved = next(x for x in result['attempts'] if x['id'] == a['id'])
            assert saved['status'] == 'solved' and saved['revision'] == a['revision'], saved
            checks = [x['result']['outcome'] for x in result['checks'] if x['attempt_id'] == a['id']]
            assert checks == ['fail','pass'], checks
            print('PASS live resume/check', exercise, a['revision'], checks, flush=True)
        (evidence / (exercise + '-export.json')).write_text(s.plain(state, 'export'))
    for a in json.loads(s.plain(state, 'export'))['attempts']:
        s.plain(state, 'forget', a['id'], '--yes')
