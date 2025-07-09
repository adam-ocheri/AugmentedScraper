"use client";

import { useState, useEffect } from "react";
import axios from "axios";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

// Get API URL from environment variables
const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
const WS_URL = process.env.NEXT_PUBLIC_WS_URL || "ws://localhost:8080";

interface TaskResult {
  url: string;
  summary?: string;
  sentiment?: string;
  status: string;
  uuid?: string;
}

interface TaskHistoryItem {
  url: string;
  uuid: string;
  status: string;
  summary?: string;
  sentiment?: string;
}

export default function Home() {
  const [loading, setLoading] = useState(false);
  const [url, setUrl] = useState("");
  const [result, setResult] = useState<TaskResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [tasks, setTasks] = useState<TaskResult[]>([]);
  const [loadingHistory, setLoadingHistory] = useState(true);

  const handleSubmit = async () => {
    if (!url.trim()) {
      setError("Please enter a valid URL");
      return;
    }

    setLoading(true);
    setError(null);
    
    try {
      const response = await axios.post(`${API_URL}/submit`, { url: url.trim() });
      console.log("Response:", response.data);
      const newTask: TaskResult = {
        url: url.trim(),
        status: "WORKING",
        uuid: response.data.uuid 
      };
      
      setTasks(prev => [newTask, ...prev]);
      setResult(newTask);
      setUrl(""); // Clear the input after submission
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : "Failed to submit URL. Please try again.";
      setError(errorMessage);
    } finally {
      setLoading(false);
    }
  };

  const fetchTaskHistory = async () => {
    try {
      setLoadingHistory(true);
      const response = await axios.get(`${API_URL}/tasks`);
      console.log("Task history response:", response.data);
      
      if (response.data.tasks && Array.isArray(response.data.tasks)) {
        const taskHistory = response.data.tasks.map((task: TaskHistoryItem) => ({
          url: task.url,
          uuid: task.uuid,
          status: task.status,
          summary: task.summary || "",
          sentiment: task.sentiment || ""
        }));
        
        setTasks(taskHistory);
        console.log("Loaded task history:", taskHistory);
      }
    } catch (err) {
      console.error("Failed to fetch task history:", err);
      // Don't show error to user for history loading, just log it
    } finally {
      setLoadingHistory(false);
    }
  };

  useEffect(() => {
    // Fetch task history on component mount
    fetchTaskHistory();
  }, []);

  useEffect(() => {
    console.log("Tasks UPDATED:", tasks);
  }, [tasks]);

  useEffect(() => {
    console.log("Result UPDATED:", result);
  }, [result]);

  useEffect(() => {
    // WebSocket connection for real-time updates
    const ws = new WebSocket(`${WS_URL}/ws`);
    
    ws.onopen = () => {
      console.log("WebSocket connected");
    };
    
    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        console.log("WebSocket message:", data);
        
        // Handle different types of messages
        if (data.type === "task_update") {
          const payload = data.payload;
          const response = JSON.parse(payload.result);
          payload.result = response;

          setTasks(prev => prev.map(task => 
            {
              if (task.uuid === data.payload.uuid) {
                console.log("Task found:", task.uuid, data.payload.uuid);
                return { ...task, status: payload.status, summary: payload.result.summary, sentiment: payload.result.sentiment };
              }
              console.log("Task not found:", task.uuid, data.payload.uuid);
              return task;
            }
          ));
          
          // Update current result if it matches
          if (result?.uuid === payload.uuid) {
            console.log("Result found:");
            console.log("result?.uuid:", result?.uuid);
            console.log("data.uuid:", payload.uuid);
            setResult(prev => prev ? { ...prev, status: payload.status, summary: payload.result.summary, sentiment: payload.result.sentiment } : null);
            // setLoading(false);
          }
          else {
            console.log("Result not found:");
            console.log("result?.uuid:", result?.uuid);
            console.log("data.uuid:", payload.result.uuid);
          }
        }
      } catch (err) {
        console.error("Error parsing WebSocket message:", err);
      }
    };
    
    ws.onerror = (error) => {
      console.error("WebSocket error:", error);
    };
    
    ws.onclose = () => {
      console.log("WebSocket disconnected");
    };
    
    return () => {
      ws.close();
    };
  }, [result?.uuid]);

  return (
    <div className="min-h-screen bg-gray-50 py-8">
      <div className="max-w-4xl mx-auto px-4">
        <div className="text-center mb-8">
          <h1 className="text-4xl font-bold text-gray-900 mb-2">
            Content Analysis System
          </h1>
          <p className="text-gray-600">
            Submit a URL to analyze and summarize web content using AI
          </p>
        </div>

        {/* URL Input Form */}
        <div className="bg-white rounded-lg shadow-md p-6 mb-8">
          <div className="flex gap-4">
            <input
              type="text"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              // onKeyPress={handleKeyPress}
              placeholder="Enter a URL to analyze (e.g., https://example.com/article)"
              className="flex-1 px-4 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              disabled={loading}
            />
            <button
              onClick={handleSubmit}
              disabled={loading || !url.trim()}
              className="px-6 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {loading ? "Processing..." : "Analyze"}
            </button>
          </div>
          
          {error && (
            <div className="mt-4 p-3 bg-red-100 border border-red-400 text-red-700 rounded-md">
              {error}
            </div>
          )}
        </div>

        {/* Current Task Result */}
        {result && (
          <div className="bg-white rounded-lg shadow-md p-6 mb-8">
            <h2 className="text-xl font-semibold mb-4">Current Task</h2>
            <div className="space-y-3">
              <div>
                <span className="font-medium text-gray-700">URL:</span>
                <a href={result.url} target="_blank" rel="noopener noreferrer" className="ml-2 text-blue-600 hover:underline">
                  {result.url}
                </a>
              </div>
              <div>
                <span className="font-medium text-gray-700">Status:</span>
                <span className={`ml-2 px-2 py-1 rounded-full text-xs font-medium ${
                  result.status === "SUCCESS" ? "bg-green-100 text-green-800" :
                  result.status === "FAILED" ? "bg-red-100 text-red-800" :
                  "bg-yellow-100 text-yellow-800"
                }`}>
                  {result.status}
                </span>
              </div>
              {result.summary && result.summary.trim() && (
                <div>
                  <span className="font-medium text-gray-700">Summary:</span>
                  <div className="mt-2 prose prose-sm max-w-none">
                    <ReactMarkdown remarkPlugins={[remarkGfm]}>{result.summary}</ReactMarkdown>
                  </div>
                </div>
              )}
              {result.sentiment && (
                <div>
                  <span className="font-medium text-gray-700">Sentiment:</span>
                  <p className="mt-1 text-gray-600">{result.sentiment}</p>
                </div>
              )}
            </div>
          </div>
        )}

        {/* Task History */}
        <div className="bg-white rounded-lg shadow-md p-6">
          <h2 className="text-xl font-semibold mb-4">Task History</h2>
          {loadingHistory ? (
            <div className="text-center py-8">
              <div className="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
              <p className="mt-2 text-gray-600">Loading task history...</p>
            </div>
          ) : tasks.length > 0 ? (
            <div className="space-y-4">
              {tasks.map((task, index) => (
                <div key={index} className="border border-gray-200 rounded-md p-4">
                  <div className="flex justify-between items-start mb-2">
                    <a href={task.url} target="_blank" rel="noopener noreferrer" className="text-blue-600 hover:underline font-medium">
                      {task.url}
                    </a>
                    <span className={`px-2 py-1 rounded-full text-xs font-medium ${
                      task.status === "SUCCESS" ? "bg-green-100 text-green-800" :
                      task.status === "FAILED" ? "bg-red-100 text-red-800" :
                      "bg-yellow-100 text-yellow-800"
                    }`}>
                      {task.status}
                    </span>
                  </div>
                  {task.summary && task.summary.trim() && (
                    <div className="mt-2 prose prose-sm max-w-none">
                      <ReactMarkdown remarkPlugins={[remarkGfm]}>{task.summary}</ReactMarkdown>
                    </div>
                  )}
                </div>
              ))}
            </div>
          ) : (
            <div className="text-center py-8 text-gray-600">
              No tasks found. Submit a URL to get started!
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
