import { useState } from 'react';
import Panel from '@/shared/Panel';
import Button from '@/shared/Button';
import { answer, search } from '@/core/api';
import type { ChatFilters } from '@/core/types';
import { logout } from '@/playground/auth.js';

const PlaygroundPage = () => {
  const [query, setQuery] = useState('');
  const [answerInput, setAnswerInput] = useState('');
  const [results, setResults] = useState<string>('');
  const [answerText, setAnswerText] = useState<string>('');

  const handleSearch = async () => {
    const data = await search(query);
    setResults(JSON.stringify(data, null, 2));
  };

  const handleAnswer = async () => {
    const filters: ChatFilters = {
      tone: 'default',
      topK: 5,
      effectiveStatus: 'active',
      domain: '',
      docType: ''
    };
    const data = await answer(answerInput, filters);
    setAnswerText(JSON.stringify(data, null, 2));
  };

  return (
    <Panel title="QA Playground">
      <div className="grid">
        <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
          <Button type="button" variant="outline" onClick={logout}>
            Logout
          </Button>
        </div>
        <label>
          <div className="label">Run /search</div>
          <textarea className="textarea" rows={3} value={query} onChange={(e) => setQuery(e.target.value)} />
        </label>
        <Button onClick={handleSearch}>Run Search</Button>
        {results && (
          <pre className="source-item" style={{ whiteSpace: 'pre-wrap' }}>
            {results}
          </pre>
        )}
        <label>
          <div className="label">Run /answer</div>
          <textarea className="textarea" rows={3} value={answerInput} onChange={(e) => setAnswerInput(e.target.value)} />
        </label>
        <Button onClick={handleAnswer}>Run Answer</Button>
        {answerText && (
          <pre className="source-item" style={{ whiteSpace: 'pre-wrap' }}>
            {answerText}
          </pre>
        )}
      </div>
    </Panel>
  );
};

export default PlaygroundPage;
