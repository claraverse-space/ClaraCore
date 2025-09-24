import React from 'react';
import { motion } from 'framer-motion';
import { 
  Button, 
  Card, 
  CardHeader, 
  CardTitle, 
  CardDescription, 
  CardContent, 
  CardFooter,
  Input,
  Modal,
  Table,
  useToast,
  ToastContainer,
  DashboardLayout
} from '../components';
import { 
  CpuChipIcon, 
  DocumentTextIcon, 
  ChartBarIcon,
  CogIcon,
  UserIcon,
  EnvelopeIcon
} from '@heroicons/react/24/outline';

const ComponentsDemo: React.FC = () => {
  const [modalOpen, setModalOpen] = React.useState(false);
  const { toasts, addToast, removeToast } = useToast();

  // Sample table data
  const tableData = [
    { id: 1, name: 'Llama 3.1 8B', type: 'Chat', size: '4.7GB', status: 'Active' },
    { id: 2, name: 'Codestral 22B', type: 'Code', size: '13.4GB', status: 'Inactive' },
    { id: 3, name: 'Mistral 7B', type: 'Chat', size: '4.1GB', status: 'Active' },
  ];

  const tableColumns = [
    { key: 'name', title: 'Model Name', dataIndex: 'name' as const },
    { key: 'type', title: 'Type', dataIndex: 'type' as const },
    { key: 'size', title: 'Size', dataIndex: 'size' as const },
    { 
      key: 'status', 
      title: 'Status', 
      dataIndex: 'status' as const,
      render: (status: string) => (
        <span className={`px-2 py-1 rounded-full text-xs font-medium ${
          status === 'Active' 
            ? 'bg-success-100 text-success-800 dark:bg-success-900/20 dark:text-success-300'
            : 'bg-neutral-100 text-neutral-800 dark:bg-neutral-800 dark:text-neutral-300'
        }`}>
          {status}
        </span>
      )
    },
  ];

  const showToast = (type: 'success' | 'error' | 'warning' | 'info') => {
    addToast({
      type,
      title: `${type.charAt(0).toUpperCase() + type.slice(1)} Toast`,
      description: `This is a ${type} message with some details.`,
      duration: 5000,
    });
  };

  const header = (
    <div className="flex items-center justify-between">
      <div className="flex items-center gap-4">
        <CpuChipIcon className="w-8 h-8 text-brand-500" />
        <div>
          <h1 className="text-2xl font-bold text-text-primary">ClaraCore UI</h1>
          <p className="text-text-secondary">Modern Component Library Demo</p>
        </div>
      </div>
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="sm">
          <CogIcon className="w-4 h-4" />
        </Button>
        <Button variant="outline" size="sm">
          Sign In
        </Button>
      </div>
    </div>
  );

  const sidebar = (
    <nav className="space-y-2">
      <div className="mb-6">
        <h2 className="text-lg font-semibold text-text-primary mb-4">Components</h2>
      </div>
      
      {[
        { icon: DocumentTextIcon, label: 'Buttons', active: true },
        { icon: ChartBarIcon, label: 'Cards', active: false },
        { icon: UserIcon, label: 'Forms', active: false },
        { icon: EnvelopeIcon, label: 'Modals', active: false },
      ].map((item) => (
        <motion.button
          key={item.label}
          className={`w-full flex items-center gap-3 px-4 py-3 rounded-lg text-left transition-colors ${
            item.active 
              ? ' text-brand-700 border border-brand-200 dark:bg-brand-900/20 dark:text-brand-300'
              : 'text-text-secondary hover:text-text-primary hover:bg-surface-secondary'
          }`}
          whileHover={{ x: 4 }}
          whileTap={{ scale: 0.98 }}
        >
          <item.icon className="w-5 h-5 flex-shrink-0" />
          <span className="font-medium">{item.label}</span>
        </motion.button>
      ))}
    </nav>
  );

  return (
    <>
      <DashboardLayout header={header} sidebar={sidebar}>
        <div className="space-y-8">
          {/* Page Header */}
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            className="mb-8"
          >
            <h1 className="text-3xl font-bold text-text-primary mb-2">Component Showcase</h1>
            <p className="text-text-secondary">
              Explore the modern UI components built for ClaraCore
            </p>
          </motion.div>

          {/* Buttons Section */}
          <Card>
            <CardHeader>
              <CardTitle>Buttons</CardTitle>
              <CardDescription>
                Various button styles and states with smooth animations
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div>
                  <h4 className="text-sm font-semibold text-text-primary mb-3">Variants</h4>
                  <div className="flex flex-wrap gap-3">
                    <Button variant="primary">Primary</Button>
                    <Button variant="secondary">Secondary</Button>
                    <Button variant="danger">Danger</Button>
                    <Button variant="ghost">Ghost</Button>
                    <Button variant="outline">Outline</Button>
                  </div>
                </div>
                <div>
                  <h4 className="text-sm font-semibold text-text-primary mb-3">Sizes</h4>
                  <div className="flex flex-wrap items-center gap-3">
                    <Button size="sm">Small</Button>
                    <Button size="md">Medium</Button>
                    <Button size="lg">Large</Button>
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Forms Section */}
          <Card>
            <CardHeader>
              <CardTitle>Form Inputs</CardTitle>
              <CardDescription>
                Input components with validation states and icons
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div className="space-y-4">
                  <Input 
                    label="Email Address" 
                    type="email" 
                    placeholder="Enter your email"
                    icon={<EnvelopeIcon className="w-5 h-5" />}
                  />
                  <Input 
                    label="Password" 
                    type="password" 
                    placeholder="Enter your password"
                  />
                  <Input 
                    label="Full Name" 
                    placeholder="Enter your name"
                    success="Valid input!"
                  />
                </div>
                <div className="space-y-4">
                  <Input 
                    label="Username" 
                    placeholder="Choose a username"
                    error="This username is already taken"
                  />
                  <Input 
                    label="Phone" 
                    placeholder="Your phone number"
                    helper="We'll never share your phone number"
                  />
                  <Input 
                    label="Disabled Field" 
                    placeholder="This field is disabled"
                    disabled
                  />
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Table Section */}
          <Card>
            <CardHeader>
              <CardTitle>Data Table</CardTitle>
              <CardDescription>
                Sortable table with animations and modern styling
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Table 
                data={tableData}
                columns={tableColumns}
                pagination={{
                  current: 1,
                  pageSize: 10,
                  total: tableData.length,
                  onChange: (page, pageSize) => console.log('Page changed:', page, pageSize)
                }}
              />
            </CardContent>
          </Card>

          {/* Interactive Elements */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <Card>
              <CardHeader>
                <CardTitle>Modal Dialog</CardTitle>
                <CardDescription>
                  Modal with backdrop blur and animations
                </CardDescription>
              </CardHeader>
              <CardContent>
                <Button onClick={() => setModalOpen(true)}>
                  Open Modal
                </Button>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Toast Notifications</CardTitle>
                <CardDescription>
                  Animated toast messages for user feedback
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="flex flex-wrap gap-2">
                  <Button 
                    variant="outline" 
                    size="sm"
                    onClick={() => showToast('success')}
                  >
                    Success
                  </Button>
                  <Button 
                    variant="outline" 
                    size="sm"
                    onClick={() => showToast('error')}
                  >
                    Error
                  </Button>
                  <Button 
                    variant="outline" 
                    size="sm"
                    onClick={() => showToast('warning')}
                  >
                    Warning
                  </Button>
                  <Button 
                    variant="outline" 
                    size="sm"
                    onClick={() => showToast('info')}
                  >
                    Info
                  </Button>
                </div>
              </CardContent>
            </Card>
          </div>

          {/* Cards Showcase */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            <Card variant="elevated" hover>
              <CardHeader>
                <CardTitle>Elevated Card</CardTitle>
                <CardDescription>Card with shadow and hover effect</CardDescription>
              </CardHeader>
              <CardContent>
                <p className="text-text-secondary">
                  This card has elevation and responds to hover interactions.
                </p>
              </CardContent>
              <CardFooter>
                <Button variant="outline" size="sm">Learn More</Button>
              </CardFooter>
            </Card>

            <Card variant="outlined">
              <CardHeader>
                <CardTitle>Outlined Card</CardTitle>
                <CardDescription>Card with border styling</CardDescription>
              </CardHeader>
              <CardContent>
                <p className="text-text-secondary">
                  A clean outlined design perfect for content sections.
                </p>
              </CardContent>
            </Card>

            <Card variant="ghost" hover>
              <CardHeader>
                <CardTitle>Ghost Card</CardTitle>
                <CardDescription>Minimal card design</CardDescription>
              </CardHeader>
              <CardContent>
                <p className="text-text-secondary">
                  Subtle styling that appears on hover.
                </p>
              </CardContent>
            </Card>
          </div>
        </div>
      </DashboardLayout>

      {/* Modal */}
      <Modal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        title="Example Modal"
        description="This is a demonstration of the modal component with backdrop blur and smooth animations."
        size="md"
      >
        <div className="space-y-4">
          <p className="text-text-secondary">
            Modal content goes here. This modal supports various sizes, 
            customizable close behavior, and smooth enter/exit animations.
          </p>
          <div className="flex gap-3 justify-end">
            <Button variant="ghost" onClick={() => setModalOpen(false)}>
              Cancel
            </Button>
            <Button variant="primary" onClick={() => setModalOpen(false)}>
              Confirm
            </Button>
          </div>
        </div>
      </Modal>

      {/* Toast Container */}
      <ToastContainer 
        toasts={toasts} 
        onRemove={removeToast}
        position="top-right"
      />
    </>
  );
};

export default ComponentsDemo;