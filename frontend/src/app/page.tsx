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
  conversation?: ConversationEntry[];
}

interface TaskHistoryItem {
  url: string;
  uuid: string;
  status: string;
  summary?: string;
  sentiment?: string;
  conversation?: ConversationEntry[];
}

interface ConversationEntry {
  role: string;
  content: string;
}

export default function Home() {
  const [loading, setLoading] = useState(false);
  const [url, setUrl] = useState("");
  const [activeArticle, setActiveArticle] = useState<TaskResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [tasks, setTasks] = useState<TaskResult[]>([]);
  const [loadingHistory, setLoadingHistory] = useState(true);
  const [sidebarOpen, setSidebarOpen] = useState(true);
  
  // Model readiness state
  const [modelsReady, setModelsReady] = useState(false);
  const [modelLoading, setModelLoading] = useState(true);
  
  // Conversation state
  const [chatMessage, setChatMessage] = useState("");
  const [chatLoading, setChatLoading] = useState(false);
  const [conversation, setConversation] = useState<ConversationEntry[]>([]);

  const handleSubmitUrl = async () => {
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
      setConversation([]); // clear the conversation
      setUrl(""); // Clear the input after submission
    } catch (err: unknown) {
      // Handle specific validation errors from the backend
      if (err && typeof err === 'object' && 'response' in err && err.response && typeof err.response === 'object' && 'status' in err.response && 'data' in err.response) {
        const axiosError = err as { response: { status: number; data: string } };
        if (axiosError.response.status === 400) {
          const errorMessage = axiosError.response.data;
          if (errorMessage.includes("Only valid https links are allowed")) {
            alert("Only valid https links are allowed! please make sure you are using a public link, such as 'https://www.example.com'");
          } else if (errorMessage.includes("The provided link is invalid or cannot be loaded")) {
            alert("The provided link is invalid or cannot be loaded! please try again later or try a different link");
          } else {
            setError(errorMessage);
          }
        } else {
          const errorMessage = err instanceof Error ? err.message : "Failed to submit URL. Please try again.";
          setError(errorMessage);
        }
      } else {
        const errorMessage = err instanceof Error ? err.message : "Failed to submit URL. Please try again.";
        setError(errorMessage);
      }
    } finally {
      setLoading(false);
    }
  };

  const handleChatSubmit = async () => {
    if (!chatMessage.trim() || !activeArticle?.uuid) {
      return;
    }

    setChatLoading(true);
    
    try {
      // Add user message to conversation immediately
      const userMessage: ConversationEntry = {
        role: "user",
        content: chatMessage
      };
      
      setConversation(prev => [...prev, userMessage]);
      
      // Also update the task's conversation data in the tasks array
      setTasks(prev => prev.map(task => {
        if (task.uuid === activeArticle.uuid) {
          const updatedConversation = [...(task.conversation || []), userMessage];
          return { ...task, conversation: updatedConversation };
        }
        return task;
      }));
      
      // Update active article conversation as well
      setActiveArticle(prev => {
        if (prev && prev.uuid === activeArticle.uuid) {
          const updatedConversation = [...(prev.conversation || []), userMessage];
          return { ...prev, conversation: updatedConversation };
        }
        return prev;
      });
      
      // Send chat request - don't add AI response here, let WebSocket handle it
      await axios.post(`${API_URL}/chat`, {
        uuid: activeArticle.uuid,
        message: chatMessage
      });

      console.log("Chat request sent, waiting for WebSocket response");
      
      // Clear input
      setChatMessage("");
      
    } catch (err) {
      console.error("Chat error:", err);
      // Remove the user message if there was an error
      setConversation(prev => prev.slice(0, -1));
      alert("Failed to send message. Please try again.");
    } finally {
      setChatLoading(false);
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
          sentiment: task.sentiment || "",
          conversation: task.conversation || []
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
    console.log("Article selected:", article);
    console.log("Article status:", article.status);
    setActiveArticle(article);
    // Load conversation for the selected article
    setConversation(article.conversation || []);
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
                return { 
                  ...task, 
                  status: payload.status, 
                  summary: payload.result.summary, 
                  sentiment: payload.result.sentiment,
                  conversation: task.conversation || [] // Preserve existing conversation data
                };
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
            setActiveArticle(prev => prev ? { 
              ...prev, 
              status: payload.status, 
              summary: payload.result.summary, 
              sentiment: payload.result.sentiment,
              conversation: prev.conversation || [] // Preserve existing conversation data
            } : null);
          }
          else {
            console.log("Active article not found or not matching:");
            console.log("activeArticle?.uuid:", activeArticle?.uuid);
            console.log("payload.uuid:", payload.uuid);
          }
        }
        
        // Handle chat response messages
        if (data.type === "chat_response") {
          const payload = data.payload;
          console.log("Chat response received:", payload);
          
          // Update conversation if it matches the active article
          if (activeArticle?.uuid === payload.uuid) {
            const assistantMessage: ConversationEntry = {
              role: "assistant",
              content: payload.response
            };
            
            setConversation(prev => {
              // Add the assistant message - WebSocket is the single source of truth
              return [...prev, assistantMessage];
            });
            
            // Also update the task's conversation data in the tasks array
            setTasks(prev => prev.map(task => {
              if (task.uuid === payload.uuid) {
                const updatedConversation = [...(task.conversation || []), assistantMessage];
                return { ...task, conversation: updatedConversation };
              }
              return task;
            }));
            
            // Update active article conversation as well
            setActiveArticle(prev => {
              if (prev && prev.uuid === payload.uuid) {
                const updatedConversation = [...(prev.conversation || []), assistantMessage];
                return { ...prev, conversation: updatedConversation };
              }
              return prev;
            });
            
            // Stop showing loading state
            setChatLoading(false);
          }
        }
        
        // Handle model readiness messages
        if (data.type === "models_ready") {
          console.log("Models ready message received:", data);
          setModelsReady(true);
          setModelLoading(false);
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
      case "done":
        return "bg-green-100 text-green-800";
      case "FAILED":
      case "failed":
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
      {/* Model Loading Overlay */}
      {modelLoading && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg p-8 max-w-md mx-4 text-center">
            <div className="inline-block animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mb-4"></div>
            <h3 className="text-lg font-semibold text-gray-900 mb-2">Loading AI Models</h3>
            <p className="text-gray-600">
              The AI models are being downloaded and loaded into memory. This may take a few minutes on first startup.
            </p>
            <div className="mt-4 text-sm text-gray-500">
              <p>Downloading: llama3.1:8b</p>
              <p>Downloading: nomic-embed-text:latest</p>
            </div>
          </div>
        </div>
      )}

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
                      {task.conversation && task.conversation.length > 0 && (
                        <p className="text-xs text-blue-600 mt-1">
                          {task.conversation.length} message{task.conversation.length !== 1 ? 's' : ''} in conversation
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
                  disabled={loading || !modelsReady}
                />
                <button
                  onClick={handleSubmitUrl}
                  disabled={loading || !url.trim() || !modelsReady}
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
              <div className="space-y-6">
                {/* Article Information */}
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

                {/* Conversation Section */}
                {(activeArticle.status === "SUCCESS" || activeArticle.status === "done") && (
                  <div className="bg-white rounded-lg shadow-md p-6">
                    <h3 className="text-lg font-semibold mb-4">Chat with AI about this article</h3>
                    
                    {/* Conversation History */}
                    <div className="mb-4 max-h-96 overflow-y-auto border border-gray-200 rounded-lg p-4 bg-gray-50">
                      {conversation.length > 0 ? (
                        <div className="space-y-4">
                          {conversation.map((message, index) => (
                            <div
                              key={index}
                              className={`flex ${message.role === 'user' ? 'justify-end' : 'justify-start'}`}
                            >
                              <div
                                className={`max-w-xs lg:max-w-md px-4 py-2 rounded-lg ${
                                  message.role === 'user'
                                    ? 'bg-blue-600 text-white'
                                    : 'bg-white text-gray-800 border border-gray-200'
                                }`}
                              >
                                <div className="text-sm font-medium mb-1">
                                  {message.role === 'user' ? 'You' : 'AI Assistant'}
                                </div>
                                <div className="text-sm whitespace-pre-wrap">
                                  {message.content}
                                </div>
                              </div>
                            </div>
                          ))}
                          {chatLoading && (
                            <div className="flex justify-start">
                              <div className="bg-white text-gray-800 border border-gray-200 px-4 py-2 rounded-lg">
                                <div className="text-sm font-medium mb-1">AI Assistant</div>
                                <div className="text-sm text-gray-500">Typing...</div>
                              </div>
                            </div>
                          )}
                        </div>
                      ) : (
                        <div className="text-center text-gray-500 py-8">
                          <svg className="w-12 h-12 mx-auto mb-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
                          </svg>
                          <p>Start a conversation about this article!</p>
                          <p className="text-sm">Ask questions about the content, summary, or sentiment.</p>
                        </div>
                      )}
                    </div>

                    {/* Chat Input */}
                    <div className="flex gap-2">
                      <input
                        type="text"
                        value={chatMessage}
                        onChange={(e) => setChatMessage(e.target.value)}
                        onKeyPress={(e) => e.key === 'Enter' && handleChatSubmit()}
                        placeholder="Ask a question about this article..."
                        className="flex-1 px-4 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                        disabled={chatLoading || !modelsReady}
                      />
                      <button
                        onClick={handleChatSubmit}
                        disabled={chatLoading || !chatMessage.trim() || !modelsReady}
                        className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
                      >
                        {chatLoading ? (
                          <div className="flex items-center">
                            <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin mr-2"></div>
                            Sending
                          </div>
                        ) : (
                          "Send"
                        )}
                      </button>
                    </div>
                  </div>
                )}
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
