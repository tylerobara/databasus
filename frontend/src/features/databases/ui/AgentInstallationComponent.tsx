import { CopyOutlined } from '@ant-design/icons';
import { App, Button, Modal, Tooltip } from 'antd';
import { useState } from 'react';

import { getApplicationServer } from '../../../constants';
import { type Database, databaseApi } from '../../../entity/databases';
import { ClipboardHelper } from '../../../shared/lib/ClipboardHelper';

type Architecture = 'amd64' | 'arm64';
type PgDeploymentType = 'system' | 'folder' | 'docker';

interface Props {
  database: Database;
  onTokenGenerated: () => void;
}

export const AgentInstallationComponent = ({ database, onTokenGenerated }: Props) => {
  const { message } = App.useApp();

  const [selectedArch, setSelectedArch] = useState<Architecture>('amd64');
  const [pgDeploymentType, setPgDeploymentType] = useState<PgDeploymentType>('system');
  const [isGenerating, setIsGenerating] = useState(false);
  const [generatedToken, setGeneratedToken] = useState<string | null>(null);

  const databasusHost = getApplicationServer();

  const handleGenerateToken = async () => {
    setIsGenerating(true);
    try {
      const result = await databaseApi.regenerateAgentToken(database.id);
      setGeneratedToken(result.token);
    } catch {
      message.error('Failed to generate token');
    } finally {
      setIsGenerating(false);
    }
  };

  const handleTokenModalClose = () => {
    setGeneratedToken(null);
    onTokenGenerated();
  };

  const copyToClipboard = async (text: string) => {
    try {
      await ClipboardHelper.copyToClipboard(text);
      message.success('Copied to clipboard');
    } catch {
      message.error('Failed to copy');
    }
  };

  const renderCodeBlock = (code: string) => (
    <div className="relative mt-2">
      <pre className="rounded-md bg-gray-900 p-4 pr-10 font-mono text-sm break-all whitespace-pre-wrap text-gray-100">
        {code}
      </pre>
      <Tooltip title="Copy">
        <button
          className="absolute top-2 right-2 cursor-pointer rounded p-1 text-gray-400 hover:text-white"
          onClick={() => copyToClipboard(code)}
        >
          <CopyOutlined />
        </button>
      </Tooltip>
    </div>
  );

  const renderTabButton = (label: string, isActive: boolean, onClick: () => void) => (
    <Button type="primary" ghost={!isActive} onClick={onClick} className="mr-2">
      {label}
    </Button>
  );

  const downloadCommand = `curl -L -o databasus-agent "${databasusHost}/api/v1/system/agent?arch=${selectedArch}" && chmod +x databasus-agent`;
  const pgHbaEntry = `host    replication   all   127.0.0.1/32   md5`;
  const grantReplicationSql = `ALTER ROLE <YOUR_PG_USER> WITH REPLICATION;`;

  // -- Step 2: Configure postgresql.conf --

  const renderStep2System = () => (
    <div>
      <div className="font-semibold dark:text-white">Step 2 — Configure postgresql.conf</div>
      <p className="mt-1 text-sm text-gray-600 dark:text-gray-400">
        Add or update these settings in your <code>postgresql.conf</code>.
      </p>
      <p className="mt-1 text-sm text-gray-600 dark:text-gray-400">
        Typical location — Debian/Ubuntu:{' '}
        <code>/etc/postgresql/&lt;version&gt;/main/postgresql.conf</code>, RHEL/CentOS:{' '}
        <code>/var/lib/pgsql/&lt;version&gt;/data/postgresql.conf</code>
      </p>
      {renderCodeBlock(`wal_level = replica
archive_mode = on
archive_command = 'cp %p /opt/databasus/wal-queue/%f.tmp && mv /opt/databasus/wal-queue/%f.tmp /opt/databasus/wal-queue/%f'`)}
      <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
        Restart PostgreSQL to apply the changes:
      </p>
      {renderCodeBlock('sudo systemctl restart postgresql')}
    </div>
  );

  const renderStep2Folder = () => (
    <div>
      <div className="font-semibold dark:text-white">Step 2 — Configure postgresql.conf</div>
      <p className="mt-1 text-sm text-gray-600 dark:text-gray-400">
        Add or update these settings in the <code>postgresql.conf</code> inside your PostgreSQL data
        directory.
      </p>
      {renderCodeBlock(`wal_level = replica
archive_mode = on
archive_command = 'cp %p /opt/databasus/wal-queue/%f.tmp && mv /opt/databasus/wal-queue/%f.tmp /opt/databasus/wal-queue/%f'`)}
      <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
        Restart PostgreSQL to apply the changes:
      </p>
      {renderCodeBlock('pg_ctl -D <YOUR_PG_DATA_DIR> restart')}
    </div>
  );

  const renderStep2Docker = () => (
    <div>
      <div className="font-semibold dark:text-white">Step 2 — Configure postgresql.conf</div>
      <p className="mt-1 text-sm text-gray-600 dark:text-gray-400">
        Add or update these settings in your <code>postgresql.conf</code> inside the container. The{' '}
        <code>/wal-queue</code> path in <code>archive_command</code> is the path{' '}
        <strong>inside the container</strong> — it must match the volume mount target configured in
        Step 5.
      </p>
      {renderCodeBlock(`wal_level = replica
archive_mode = on
archive_command = 'cp %p /wal-queue/%f.tmp && mv /wal-queue/%f.tmp /wal-queue/%f'`)}
      <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
        Restart the container to apply the changes:
      </p>
      {renderCodeBlock('docker restart <CONTAINER_NAME>')}
    </div>
  );

  // -- Step 3: Configure pg_hba.conf --

  const renderStep3System = () => (
    <div>
      <div className="font-semibold dark:text-white">Step 3 — Configure pg_hba.conf</div>
      <p className="mt-1 text-sm text-gray-600 dark:text-gray-400">
        Add this line to <code>pg_hba.conf</code> to allow <code>pg_basebackup</code> to take full
        backups via a local replication connection. Adjust the address and auth method as needed.
      </p>
      <p className="mt-1 text-sm text-gray-600 dark:text-gray-400">
        Typical location — Debian/Ubuntu:{' '}
        <code>/etc/postgresql/&lt;version&gt;/main/pg_hba.conf</code>, RHEL/CentOS:{' '}
        <code>/var/lib/pgsql/&lt;version&gt;/data/pg_hba.conf</code>
      </p>
      {renderCodeBlock(pgHbaEntry)}
      <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
        Restart PostgreSQL to apply the changes:
      </p>
      {renderCodeBlock('sudo systemctl restart postgresql')}
    </div>
  );

  const renderStep3Folder = () => (
    <div>
      <div className="font-semibold dark:text-white">Step 3 — Configure pg_hba.conf</div>
      <p className="mt-1 text-sm text-gray-600 dark:text-gray-400">
        Add this line to <code>pg_hba.conf</code> in your PostgreSQL data directory to allow{' '}
        <code>pg_basebackup</code> to take full backups via a local replication connection. Adjust
        the address and auth method as needed.
      </p>
      {renderCodeBlock(pgHbaEntry)}
      <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
        Restart PostgreSQL to apply the changes:
      </p>
      {renderCodeBlock('pg_ctl -D <YOUR_PG_DATA_DIR> restart')}
    </div>
  );

  const renderStep3Docker = () => (
    <div>
      <div className="font-semibold dark:text-white">Step 3 — Configure pg_hba.conf</div>
      <p className="mt-1 text-sm text-gray-600 dark:text-gray-400">
        Add this line to <code>pg_hba.conf</code> inside the container to allow{' '}
        <code>pg_basebackup</code> to take full backups via a replication connection on the
        container&apos;s loopback interface. Adjust the address and auth method as needed.
      </p>
      {renderCodeBlock(pgHbaEntry)}
      <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
        Restart the container to apply the changes:
      </p>
      {renderCodeBlock('docker restart <CONTAINER_NAME>')}
    </div>
  );

  // -- Step 5: WAL queue directory --

  const renderStep5System = () => (
    <div>
      <div className="font-semibold dark:text-white">Step 5 — Create WAL queue directory</div>
      <p className="mt-1 text-sm text-gray-600 dark:text-gray-400">
        PostgreSQL will place WAL archive files here for the agent to upload.
      </p>
      {renderCodeBlock('mkdir -p /opt/databasus/wal-queue')}
      <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
        Ensure the directory is writable by PostgreSQL and readable by the agent:
      </p>
      {renderCodeBlock(`chown postgres:postgres /opt/databasus/wal-queue
chmod 755 /opt/databasus/wal-queue`)}
    </div>
  );

  const renderStep5Folder = () => (
    <div>
      <div className="font-semibold dark:text-white">Step 5 — Create WAL queue directory</div>
      <p className="mt-1 text-sm text-gray-600 dark:text-gray-400">
        PostgreSQL will place WAL archive files here for the agent to upload.
      </p>
      {renderCodeBlock('mkdir -p /opt/databasus/wal-queue')}
      <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
        Ensure the directory is writable by PostgreSQL and readable by the agent:
      </p>
      {renderCodeBlock(`chown postgres:postgres /opt/databasus/wal-queue
chmod 755 /opt/databasus/wal-queue`)}
    </div>
  );

  const renderStep5Docker = () => (
    <div>
      <div className="font-semibold dark:text-white">Step 5 — Set up WAL queue volume</div>
      <p className="mt-1 text-sm text-gray-600 dark:text-gray-400">
        The WAL queue directory must be a <strong>volume mount</strong> shared between the
        PostgreSQL container and the host. The agent reads WAL files from the host path, while
        PostgreSQL writes to the container path via <code>archive_command</code>.
      </p>
      {renderCodeBlock('mkdir -p /opt/databasus/wal-queue')}
      <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
        Then mount it as a volume so both the container and the agent can access it:
      </p>
      {renderCodeBlock(`# In your docker run command:
docker run ... -v /opt/databasus/wal-queue:/wal-queue ...

# Or in docker-compose.yml:
volumes:
  - /opt/databasus/wal-queue:/wal-queue`)}
      <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
        Ensure the directory inside the container is owned by the <code>postgres</code> user:
      </p>
      {renderCodeBlock(`# Inside the container (or via docker exec):
chown postgres:postgres /wal-queue`)}
    </div>
  );

  // -- Step 6: Start the agent --

  const buildBaseFlags = () => [
    `  --databasus-host=${databasusHost} \\`,
    `  --db-id=${database.id} \\`,
    `  --token=<YOUR_AGENT_TOKEN> \\`,
    `  --pg-host=localhost \\`,
    `  --pg-port=5432 \\`,
    `  --pg-user=<YOUR_PG_USER> \\`,
    `  --pg-password=<YOUR_PG_PASSWORD> \\`,
  ];

  const renderStep6System = () => {
    const startCommand = [
      './databasus-agent start \\',
      ...buildBaseFlags(),
      `  --pg-type=host \\`,
      `  --pg-wal-dir=/opt/databasus/wal-queue`,
    ].join('\n');

    return (
      <div>
        <div className="font-semibold dark:text-white">Step 6 — Start the agent</div>
        <p className="mt-1 text-sm text-gray-600 dark:text-gray-400">
          Replace placeholders in <code>{'<ANGLE_BRACKETS>'}</code> with your actual values.
        </p>
        {renderCodeBlock(startCommand)}
      </div>
    );
  };

  const renderStep6Folder = () => {
    const startCommand = [
      './databasus-agent start \\',
      ...buildBaseFlags(),
      `  --pg-type=host \\`,
      `  --pg-host-bin-dir=<PATH_TO_PG_BIN_DIR> \\`,
      `  --pg-wal-dir=/opt/databasus/wal-queue`,
    ].join('\n');

    return (
      <div>
        <div className="font-semibold dark:text-white">Step 6 — Start the agent</div>
        <p className="mt-1 text-sm text-gray-600 dark:text-gray-400">
          Replace placeholders in <code>{'<ANGLE_BRACKETS>'}</code> with your actual values.{' '}
          <code>--pg-host-bin-dir</code> should point to the directory containing{' '}
          <code>pg_basebackup</code> (e.g. <code>/usr/lib/postgresql/17/bin</code>).
        </p>
        {renderCodeBlock(startCommand)}
      </div>
    );
  };

  const renderStep6Docker = () => {
    const startCommand = [
      './databasus-agent start \\',
      ...buildBaseFlags(),
      `  --pg-type=docker \\`,
      `  --pg-docker-container-name=<CONTAINER_NAME> \\`,
      `  --pg-wal-dir=/opt/databasus/wal-queue`,
    ].join('\n');

    return (
      <div>
        <div className="font-semibold dark:text-white">Step 6 — Start the agent</div>
        <p className="mt-1 text-sm text-gray-600 dark:text-gray-400">
          Replace placeholders in <code>{'<ANGLE_BRACKETS>'}</code> with your actual values.
        </p>
        <p className="mt-1 text-sm text-amber-600 dark:text-amber-400">
          Use the PostgreSQL port <strong>inside the container</strong> (usually 5432), not the
          host-mapped port.
        </p>
        {renderCodeBlock(startCommand)}
      </div>
    );
  };

  // -- Dispatch helpers --

  const renderStep2 = () => {
    if (pgDeploymentType === 'system') return renderStep2System();
    if (pgDeploymentType === 'folder') return renderStep2Folder();
    return renderStep2Docker();
  };

  const renderStep3 = () => {
    if (pgDeploymentType === 'system') return renderStep3System();
    if (pgDeploymentType === 'folder') return renderStep3Folder();
    return renderStep3Docker();
  };

  const renderStep5 = () => {
    if (pgDeploymentType === 'system') return renderStep5System();
    if (pgDeploymentType === 'folder') return renderStep5Folder();
    return renderStep5Docker();
  };

  const renderStep6 = () => {
    if (pgDeploymentType === 'system') return renderStep6System();
    if (pgDeploymentType === 'folder') return renderStep6Folder();
    return renderStep6Docker();
  };

  return (
    <div className="min-w-0 rounded-tr-md rounded-br-md rounded-bl-md bg-white p-3 shadow md:p-5 dark:bg-gray-800">
      <h2 className="text-lg font-bold md:text-xl dark:text-white">Agent installation</h2>

      <div className="mt-2 flex items-center text-sm text-gray-500 dark:text-gray-400">
        <span className="mr-1">Database ID:</span>
        <code className="rounded bg-gray-100 px-2 py-0.5 text-xs dark:bg-gray-700">
          {database.id}
        </code>
        <Tooltip title="Copy">
          <button
            className="ml-1 cursor-pointer rounded p-1 text-gray-400 hover:text-gray-700 dark:hover:text-white"
            onClick={() => copyToClipboard(database.id)}
          >
            <CopyOutlined style={{ fontSize: 12 }} />
          </button>
        </Tooltip>
      </div>

      <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
        WAL backup mode requires the Databasus agent to be installed on the server where PostgreSQL
        runs. Follow the steps below to set it up.
      </p>

      <p className="mt-1 text-sm text-amber-600 dark:text-amber-400">
        Requires PostgreSQL 15 or newer.
      </p>

      <div className="mt-5">
        <div className="mb-1 text-sm font-medium text-gray-700 dark:text-gray-300">
          Architecture
        </div>
        <div className="flex">
          {renderTabButton('amd64', selectedArch === 'amd64', () => setSelectedArch('amd64'))}
          {renderTabButton('arm64', selectedArch === 'arm64', () => setSelectedArch('arm64'))}
        </div>
      </div>

      <div className="mt-4">
        <div className="mb-1 text-sm font-medium text-gray-700 dark:text-gray-300">
          PostgreSQL installation type
        </div>
        <div className="flex">
          {renderTabButton('System-wide', pgDeploymentType === 'system', () =>
            setPgDeploymentType('system'),
          )}
          {renderTabButton('Specific folder', pgDeploymentType === 'folder', () =>
            setPgDeploymentType('folder'),
          )}
          {renderTabButton('Docker', pgDeploymentType === 'docker', () =>
            setPgDeploymentType('docker'),
          )}
        </div>
        <div className="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {pgDeploymentType === 'system' &&
            'pg_basebackup is available in the system PATH (default PostgreSQL install)'}
          {pgDeploymentType === 'folder' &&
            'pg_basebackup is in a specific directory (e.g. /usr/lib/postgresql/17/bin)'}
          {pgDeploymentType === 'docker' && 'PostgreSQL runs inside a Docker container'}
        </div>
      </div>

      <div className="mt-6">
        <div className="font-semibold dark:text-white">Agent token</div>
        {database.isAgentTokenGenerated ? (
          <div className="mt-2">
            <p className="mb-2 text-sm text-amber-600 dark:text-amber-400">
              A token has already been generated. Regenerating will invalidate the existing one.
            </p>
            <Button danger loading={isGenerating} onClick={handleGenerateToken}>
              Regenerate token
            </Button>
          </div>
        ) : (
          <div className="mt-2">
            <p className="mb-2 text-sm text-gray-600 dark:text-gray-400">
              Generate a token the agent will use to authenticate with Databasus.
            </p>
            <Button type="primary" loading={isGenerating} onClick={handleGenerateToken}>
              Generate token
            </Button>
          </div>
        )}
      </div>

      <Modal
        title="Agent Token"
        open={generatedToken !== null}
        onCancel={handleTokenModalClose}
        footer={
          <Button type="primary" onClick={handleTokenModalClose}>
            I&apos;ve saved the token
          </Button>
        }
      >
        {renderCodeBlock(generatedToken ?? '')}
        <p className="mt-3 text-sm text-amber-600 dark:text-amber-400">
          This token will only be shown once. Store it securely — you won&apos;t be able to retrieve
          it again.
        </p>
      </Modal>

      <div className="mt-6 space-y-6">
        <div>
          <div className="font-semibold dark:text-white">Step 1 — Download the agent</div>
          {renderCodeBlock(downloadCommand)}
        </div>

        {renderStep2()}
        {renderStep3()}

        <div>
          <div className="font-semibold dark:text-white">Step 4 — Grant replication privilege</div>
          <p className="mt-1 text-sm text-gray-600 dark:text-gray-400">
            This is a PostgreSQL requirement for running <code>pg_basebackup</code> — it does not
            set up a replica.
          </p>
          {renderCodeBlock(grantReplicationSql)}
        </div>

        {renderStep5()}
        {renderStep6()}

        <div>
          <div className="font-semibold dark:text-white">After installation</div>
          <ul className="mt-1 list-disc space-y-1 pl-5 text-sm text-gray-600 dark:text-gray-400">
            <li>
              The agent runs in the background after <code>start</code>
            </li>
            <li>
              Check status: <code>./databasus-agent status</code>
            </li>
            <li>
              View logs: <code>databasus.log</code> in the working directory
            </li>
            <li>
              Stop the agent: <code>./databasus-agent stop</code>
            </li>
          </ul>
        </div>
      </div>
    </div>
  );
};
