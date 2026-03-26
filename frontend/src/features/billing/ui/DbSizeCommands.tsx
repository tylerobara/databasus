import { useState } from 'react';

interface DbSizeCommand {
  label: string;
  code: string;
}

interface Props {
  commands: DbSizeCommand[];
}

export function DbSizeCommands({ commands }: Props) {
  const [copiedIndex, setCopiedIndex] = useState<number | null>(null);

  return (
    <details className="group mb-2">
      <summary className="flex cursor-pointer list-none items-center gap-1.5 text-gray-500 transition-colors hover:text-gray-400">
        <svg
          width="12"
          height="12"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          className="transition-transform group-open:rotate-90"
        >
          <path d="M9 18l6-6-6-6" />
        </svg>
        How to check DB size?
      </summary>

      <div className="mt-2 space-y-1.5">
        {commands.map((cmd, index) => (
          <div key={index}>
            <p className="mb-1 text-xs text-gray-400">{cmd.label}</p>
            <div className="relative">
              <pre className="overflow-x-auto rounded-lg border border-[#ffffff20] bg-[#1f2937] px-2.5 py-1.5 pr-16 text-xs">
                <code className="block whitespace-pre text-gray-300">{cmd.code}</code>
              </pre>
              <button
                onClick={async () => {
                  try {
                    await navigator.clipboard.writeText(cmd.code);
                    setCopiedIndex(index);
                    setTimeout(() => setCopiedIndex(null), 2000);
                  } catch {
                    /* ignore */
                  }
                }}
                className={`absolute top-2 right-2 rounded border border-[#ffffff20] px-2 py-0.5 text-white transition-colors ${
                  copiedIndex === index ? 'bg-green-500' : 'bg-blue-600 hover:bg-blue-700'
                }`}
              >
                {copiedIndex === index ? 'Copied!' : 'Copy'}
              </button>
            </div>
          </div>
        ))}
      </div>
    </details>
  );
}
