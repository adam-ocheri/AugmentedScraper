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
  const [activeArticle, setActiveArticle] = useState<TaskResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [tasks, setTasks] = useState<TaskResult[]>([]);
  const [loadingHistory, setLoadingHistory] = useState(true);
  const [sidebarOpen, setSidebarOpen] = useState(true);

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

      // validate that the task is pending and no error occured
      if (response.data.status !== "pending" && response.data.status !== "processing" && response.data.status !== "done") {
        console.log("Wrong status type! response.data.status: ", response.data.status);
        alert("Error occurred while processing the article. Please try again.");
        setLoading(false);
        return;
      }
      
      // validate not an existing task or finished
      else if (response.data.status === "processing") {
        alert("Article already being processed! Please wait for it to finish.");
        setLoading(false);
        return;
      }
      else if (response.data.status === "done") {
        alert("Article already exists! It is available in your history.");
        setLoading(false);
        return;
      }

      const newTask: TaskResult = {
        url: url.trim(),
        status: "WORKING",
        uuid: response.data.uuid 
      };
      
      setTasks(prev => [newTask, ...prev]);
      setActiveArticle(newTask); // Set as active article
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

  const handleArticleSelect = (article: TaskResult) => {
    setActiveArticle(article);
  };

  useEffect(() => {
    // Fetch task history on component mount
    fetchTaskHistory();
  }, []);

  useEffect(() => {
    console.log("Tasks UPDATED:", tasks);
  }, [tasks]);

  useEffect(() => {
    console.log("Active Article UPDATED:", activeArticle);
  }, [activeArticle]);

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
          
          // Update active article only if it matches the current active article
          if (activeArticle?.uuid === payload.uuid) {
            console.log("Active article found and updating:");
            console.log("activeArticle?.uuid:", activeArticle?.uuid);
            console.log("payload.uuid:", payload.uuid);
            setActiveArticle(prev => prev ? { ...prev, status: payload.status, summary: payload.result.summary, sentiment: payload.result.sentiment } : null);
          }
          else {
            console.log("Active article not found or not matching:");
            console.log("activeArticle?.uuid:", activeArticle?.uuid);
            console.log("payload.uuid:", payload.uuid);
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
  }, [activeArticle?.uuid]);

  const getStatusColor = (status: string) => {
    switch (status) {
      case "SUCCESS":
        return "bg-green-100 text-green-800";
      case "FAILED":
        return "bg-red-100 text-red-800";
      default:
        return "bg-yellow-100 text-yellow-800";
    }
  };

  const truncateUrl = (url: string) => {
    if (url.length > 40) {
      return url.substring(0, 40) + "...";
    }
    return url;
  };

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Mobile sidebar toggle */}
      <div className="lg:hidden fixed top-4 left-4 z-50">
        <button
          onClick={() => setSidebarOpen(!sidebarOpen)}
          className="p-2 bg-white rounded-md shadow-md"
        >
          <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
          </svg>
        </button>
      </div>

      <div className="flex">
        {/* Sidebar */}
        <div className={`fixed lg:static inset-y-0 left-0 z-40 w-80 bg-white shadow-lg transform ${sidebarOpen ? 'translate-x-0' : '-translate-x-full'} lg:translate-x-0 transition-transform duration-300 ease-in-out`}>
          <div className="h-full flex flex-col">
            {/* Sidebar Header */}
            <div className="p-4 border-b border-gray-200">
              <h2 className="text-lg font-semibold text-gray-900">Task History</h2>
              <p className="text-sm text-gray-600 mt-1">Click to view details</p>
            </div>

            {/* Sidebar Content */}
            <div className="flex-1 overflow-y-auto">
              {loadingHistory ? (
                <div className="flex items-center justify-center p-8">
                  <div className="inline-block animate-spin rounded-full h-6 w-6 border-b-2 border-blue-600"></div>
                  <span className="ml-2 text-gray-600">Loading...</span>
                </div>
              ) : tasks.length > 0 ? (
                <div className="p-4 space-y-2">
                  {tasks.map((task, index) => (
                    <button
                      key={index}
                      onClick={() => handleArticleSelect(task)}
                      className={`w-full text-left p-3 rounded-lg border transition-all duration-200 hover:shadow-md ${
                        activeArticle?.uuid === task.uuid
                          ? 'border-blue-500 bg-blue-50 shadow-md'
                          : 'border-gray-200 bg-white hover:border-gray-300'
                      }`}
                    >
                      <div className="flex items-start justify-between mb-2">
                        <span className="text-sm font-medium text-gray-900 truncate">
                          {truncateUrl(task.url)}
                        </span>
                        <span className={`px-2 py-1 rounded-full text-xs font-medium flex-shrink-0 ml-2 ${getStatusColor(task.status)}`}>
                          {task.status}
                        </span>
                      </div>
                      {task.summary && task.summary.trim() && (
                        <p className="text-xs text-gray-600 line-clamp-2">
                          {task.summary.substring(0, 100)}...
                        </p>
                      )}
                    </button>
                  ))}
                </div>
              ) : (
                <div className="flex items-center justify-center p-8 text-gray-600">
                  <div className="text-center">
                    <svg className="w-12 h-12 mx-auto mb-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                    </svg>
                    <p>No tasks found</p>
                    <p className="text-sm">Submit a URL to get started!</p>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Main Content */}
        <div className="flex-1 lg:ml-0">
          <div className="max-w-4xl mx-auto px-4 py-8">
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

            {/* Active Article Details */}
            {activeArticle ? (
              <div className="bg-white rounded-lg shadow-md p-6">
                <h2 className="text-xl font-semibold mb-4">Article Details</h2>
                <div className="space-y-3">
                  <div>
                    <span className="font-medium text-gray-700">URL:</span>
                    <a href={activeArticle.url} target="_blank" rel="noopener noreferrer" className="ml-2 text-blue-600 hover:underline">
                      {activeArticle.url}
                    </a>
                  </div>
                  <div>
                    <span className="font-medium text-gray-700">Status:</span>
                    <span className={`ml-2 px-2 py-1 rounded-full text-xs font-medium ${getStatusColor(activeArticle.status)}`}>
                      {activeArticle.status}
                    </span>
                  </div>
                  {activeArticle.summary && activeArticle.summary.trim() && (
                    <div>
                      <span className="font-medium text-gray-700">Summary:</span>
                      <div className="mt-2 prose prose-sm max-w-none">
                        <ReactMarkdown remarkPlugins={[remarkGfm]}>{activeArticle.summary}</ReactMarkdown>
                      </div>
                    </div>
                  )}
                  {activeArticle.sentiment && (
                    <div>
                      <span className="font-medium text-gray-700">Sentiment:</span>
                      <p className="mt-1 text-gray-600">{activeArticle.sentiment}</p>
                    </div>
                  )}
                </div>
              </div>
            ) : (
              <div className="bg-white rounded-lg shadow-md p-6 text-center">
                <div className="text-gray-600">
                  <svg className="w-16 h-16 mx-auto mb-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                  </svg>
                  <h3 className="text-lg font-medium mb-2">No Article Selected</h3>
                  <p>Select an article from the sidebar or submit a new URL to get started.</p>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Mobile overlay */}
      {sidebarOpen && (
        <div 
          className="fixed inset-0 bg-black bg-opacity-50 z-30 lg:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}
    </div>
  );
}
